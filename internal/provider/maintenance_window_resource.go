package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

var (
	_ resource.Resource                = (*maintenanceWindowResource)(nil)
	_ resource.ResourceWithConfigure   = (*maintenanceWindowResource)(nil)
	_ resource.ResourceWithImportState = (*maintenanceWindowResource)(nil)
)

type maintenanceWindowResource struct {
	api *client.Client
}

type maintenanceWindowModel struct {
	ID             types.String      `tfsdk:"id"`
	Title          types.String      `tfsdk:"title"`
	Description    types.String      `tfsdk:"description"`
	StartAt        timetypes.RFC3339 `tfsdk:"start_at"`
	EndAt          timetypes.RFC3339 `tfsdk:"end_at"`
	Recurrence     types.String      `tfsdk:"recurrence"`
	RecurrenceEnd  timetypes.RFC3339 `tfsdk:"recurrence_end"`
	CheckUIDs      types.Set         `tfsdk:"check_uids"`
	CheckGroupUIDs types.Set         `tfsdk:"check_group_uids"`
}

func newMaintenanceWindowResource() resource.Resource { return &maintenanceWindowResource{} }

func (r *maintenanceWindowResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_maintenance_window"
}

func (r *maintenanceWindowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A maintenance window: a planned period, possibly recurring, during which the " +
			"selected checks do not open incidents.\n\n" +
			"-> Removing `description` or `recurrence_end` from the configuration needs a SolidPing release " +
			"after v0.52.0; older servers keep the previous value.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UID of the maintenance window.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"title": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Title of the maintenance.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Details about the maintenance.",
			},
			"start_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Required:            true,
				MarkdownDescription: "Start of the (first) window, as an RFC 3339 timestamp, e.g. `2027-01-15T22:00:00Z`.",
			},
			"end_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Required:            true,
				MarkdownDescription: "End of the (first) window, as an RFC 3339 timestamp.",
			},
			"recurrence": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("none"),
				MarkdownDescription: "`none` (the default), `daily`, `weekly` or `monthly`.",
				Validators:          []validator.String{stringvalidator.OneOf("none", "daily", "weekly", "monthly")},
			},
			"recurrence_end": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Optional:            true,
				MarkdownDescription: "When a recurring window stops repeating, as an RFC 3339 timestamp.",
			},
			"check_uids": schema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "UIDs of the checks under maintenance.",
			},
			"check_group_uids": schema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "UIDs of the check groups under maintenance.",
			},
		},
	}
}

func (r *maintenanceWindowResource) Configure(
	_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse,
) {
	r.api = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *maintenanceWindowResource) payload(plan *maintenanceWindowModel, update bool) client.Payload {
	body := client.Payload{
		"title":      plan.Title.ValueString(),
		"startAt":    plan.StartAt.ValueString(),
		"endAt":      plan.EndAt.ValueString(),
		"recurrence": plan.Recurrence.ValueString(),
	}

	putString(body, "description", plan.Description)

	if !plan.RecurrenceEnd.IsNull() && !plan.RecurrenceEnd.IsUnknown() {
		body["recurrenceEnd"] = plan.RecurrenceEnd.ValueString()
	}

	if update {
		// An explicit null clears the field (an omitted key leaves it as is).
		if plan.Description.IsNull() {
			body["description"] = nil
		}

		if plan.RecurrenceEnd.IsNull() {
			body["recurrenceEnd"] = nil
		}
	}

	return body
}

func stringSet(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	out := []string{}

	if !set.IsNull() && !set.IsUnknown() {
		diags.Append(set.ElementsAs(ctx, &out, false)...)
	}

	sort.Strings(out)

	return out
}

// setChecks replaces the checks and check groups attached to the window.
func (r *maintenanceWindowResource) setChecks(
	ctx context.Context, uid string, plan *maintenanceWindowModel, diags *diag.Diagnostics,
) {
	checks := stringSet(ctx, plan.CheckUIDs, diags)
	groups := stringSet(ctx, plan.CheckGroupUIDs, diags)

	if diags.HasError() {
		return
	}

	if err := r.api.SetMaintenanceWindowChecks(ctx, uid, checks, groups); err != nil {
		addAPIError(diags, "set the checks of the maintenance window", err)
	}
}

func rfc3339OrNull(value string, prior timetypes.RFC3339) timetypes.RFC3339 {
	if value == "" {
		return timetypes.NewRFC3339Null()
	}

	// Same instant, another spelling (the server normalizes to UTC): keep
	// the configured spelling.
	if !prior.IsNull() && !prior.IsUnknown() {
		priorTime, d1 := prior.ValueRFC3339Time()
		serverTime, d2 := timetypes.NewRFC3339ValueMust(value).ValueRFC3339Time()

		if !d1.HasError() && !d2.HasError() && priorTime.Equal(serverTime) {
			return prior
		}
	}

	return timetypes.NewRFC3339ValueMust(value)
}

func (m *maintenanceWindowModel) apply(window *client.MaintenanceWindow) {
	m.ID = types.StringValue(window.UID)
	m.Title = types.StringValue(window.Title)
	m.Description = optionalString(window.Description, m.Description)
	m.StartAt = rfc3339OrNull(window.StartAt, m.StartAt)
	m.EndAt = rfc3339OrNull(window.EndAt, m.EndAt)
	m.Recurrence = types.StringValue(window.Recurrence)
	m.RecurrenceEnd = rfc3339OrNull(window.RecurrenceEnd, m.RecurrenceEnd)
}

func uidSet(ctx context.Context, uids []string, prior types.Set, diags *diag.Diagnostics) types.Set {
	if len(uids) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.SetNull(types.StringType)
	}

	if uids == nil {
		uids = []string{}
	}

	set, d := types.SetValueFrom(ctx, types.StringType, uids)
	diags.Append(d...)

	return set
}

func (r *maintenanceWindowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan maintenanceWindowModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	window, err := r.api.CreateMaintenanceWindow(ctx, r.payload(&plan, false))
	if err != nil {
		addAPIError(&resp.Diagnostics, "create the maintenance window", err)

		return
	}

	plan.apply(window)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	// The checks are attached with a second call: the state is already saved,
	// so a failure here leaves a window Terraform knows about.
	r.setChecks(ctx, window.UID, &plan, &resp.Diagnostics)
}

func (r *maintenanceWindowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state maintenanceWindowModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	window, err := r.api.GetMaintenanceWindow(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		addAPIError(&resp.Diagnostics, "read the maintenance window", err)

		return
	}

	links, err := r.api.ListMaintenanceWindowChecks(ctx, window.UID)
	if err != nil {
		addAPIError(&resp.Diagnostics, "read the checks of the maintenance window", err)

		return
	}

	var checks, groups []string

	for i := range links {
		if links[i].CheckUID != "" {
			checks = append(checks, links[i].CheckUID)
		}

		if links[i].CheckGroupUID != "" {
			groups = append(groups, links[i].CheckGroupUID)
		}
	}

	state.apply(window)
	state.CheckUIDs = uidSet(ctx, checks, state.CheckUIDs, &resp.Diagnostics)
	state.CheckGroupUIDs = uidSet(ctx, groups, state.CheckGroupUIDs, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *maintenanceWindowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state maintenanceWindowModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	window, err := r.api.UpdateMaintenanceWindow(ctx, state.ID.ValueString(), r.payload(&plan, true))
	if err != nil {
		addAPIError(&resp.Diagnostics, "update the maintenance window", err)

		return
	}

	if !plan.CheckUIDs.Equal(state.CheckUIDs) || !plan.CheckGroupUIDs.Equal(state.CheckGroupUIDs) {
		r.setChecks(ctx, window.UID, &plan, &resp.Diagnostics)

		if resp.Diagnostics.HasError() {
			return
		}
	}

	plan.apply(window)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *maintenanceWindowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state maintenanceWindowModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.api.DeleteMaintenanceWindow(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		addAPIError(&resp.Diagnostics, "delete the maintenance window", err)
	}
}

func (r *maintenanceWindowResource) ImportState(
	ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	importByOrgUID(ctx, r.api, req, resp)
}
