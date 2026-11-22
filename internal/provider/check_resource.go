package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

var (
	_ resource.Resource                = (*checkResource)(nil)
	_ resource.ResourceWithConfigure   = (*checkResource)(nil)
	_ resource.ResourceWithImportState = (*checkResource)(nil)
)

var periodRegexp = regexp.MustCompile(`^\d{2,}:[0-5]\d:[0-5]\d$`)

type checkResource struct {
	api *client.Client
}

type checkModel struct {
	ID                  types.String         `tfsdk:"id"`
	Name                types.String         `tfsdk:"name"`
	Slug                types.String         `tfsdk:"slug"`
	Description         types.String         `tfsdk:"description"`
	Type                types.String         `tfsdk:"type"`
	Config              jsontypes.Normalized `tfsdk:"config"`
	ConfigSensitive     jsontypes.Normalized `tfsdk:"config_sensitive"`
	Enabled             types.Bool           `tfsdk:"enabled"`
	Period              types.String         `tfsdk:"period"`
	Regions             types.List           `tfsdk:"regions"`
	Placement           types.String         `tfsdk:"placement"`
	RegionCount         types.Int64          `tfsdk:"region_count"`
	Labels              types.Map            `tfsdk:"labels"`
	CheckGroupUID       types.String         `tfsdk:"check_group_uid"`
	EscalationPolicyUID types.String         `tfsdk:"escalation_policy_uid"`
}

func newCheckResource() resource.Resource { return &checkResource{} }

func (r *checkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

func (r *checkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A monitoring check (HTTP, TCP, DNS, heartbeat, ...).\n\n" +
			"The type-specific settings go in `config`, as a JSON object. Secrets (passwords, tokens, " +
			"basic-auth credentials) go in `config_sensitive`: the API stores them encrypted and never " +
			"returns them, so the provider keeps them in the state as written and never reports drift on them.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UID of the check.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Display name. Derived from the target by the server when omitted.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slug": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "URL-friendly identifier, unique in the organization. Derived by the server when omitted.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "What this check monitors.",
			},
			"type": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Check type: `http`, `tcp`, `icmp`, `dns`, `ssl`, `heartbeat`, `domain`, ... " +
					"(the full list is in the SolidPing documentation). Changing it replaces the check.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				Required:   true,
				MarkdownDescription: "Type-specific configuration, as a JSON object: `jsonencode({ url = \"https://acme.com\" })`. " +
					"Only the keys written here are compared with the server, so defaults the server adds never show as drift.",
			},
			"config_sensitive": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
				Sensitive:  true,
				MarkdownDescription: "Secret configuration keys, as a JSON object merged over `config` when the check is " +
					"written. Never read back from the server. Removing a key clears the stored secret.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the check runs. Defaults to `true`.",
			},
			"period": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Interval between two runs, as `HH:MM:SS` (e.g. `00:01:00`). Defaults to the server default (one minute).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(periodRegexp, "must be a duration written as HH:MM:SS, e.g. 00:01:00"),
				},
			},
			"regions": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				MarkdownDescription: "Region slugs the check runs from. Setting it pins the check to these regions. " +
					"When omitted, the server places the check and this attribute reports the current placement.",
			},
			"placement": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "`pinned` (the `regions` list never moves) or `auto` (the scheduler picks " +
					"`region_count` healthy regions). Derived from `regions` when omitted.",
				Validators: []validator.String{stringvalidator.OneOf("pinned", "auto")},
			},
			"region_count": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Automatic placement only: how many regions run the check.",
				Validators:          []validator.Int64{int64validator.AtLeast(1)},
			},
			"labels": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Key-value labels, used to filter checks and to feed status-page section selectors.",
			},
			"check_group_uid": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "UID of the check group this check belongs to.",
			},
			"escalation_policy_uid": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "UID of the escalation policy assigned to this check. " +
					"When omitted, the check inherits its group's policy, then the organization default.",
			},
		},
	}
}

func (r *checkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.api = configureClient(req.ProviderData, &resp.Diagnostics)
}

// payload builds the create/update body. config is the raw configuration: the
// placement attributes are only sent when the user wrote them, because their
// planned values are otherwise just what the scheduler chose last.
func (r *checkResource) payload(
	ctx context.Context, plan, config, state *checkModel, diags *diag.Diagnostics,
) client.Payload {
	body := client.Payload{}

	putString(body, "name", plan.Name)
	putString(body, "slug", plan.Slug)
	putBool(body, "enabled", plan.Enabled)
	putString(body, "period", plan.Period)
	putString(body, "placement", config.Placement)
	putInt(body, "regionCount", config.RegionCount)

	if !config.Regions.IsNull() && !config.Regions.IsUnknown() {
		var regions []string

		diags.Append(plan.Regions.ElementsAs(ctx, &regions, false)...)

		body["regions"] = regions
	}

	labels := map[string]string{}

	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		diags.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
	}

	var priorSensitive map[string]any

	if state != nil {
		priorSensitive = decodeJSONObject(state.ConfigSensitive, "config_sensitive", diags)
	}

	body["config"] = mergeSecrets(
		decodeJSONObject(plan.Config, "config", diags),
		decodeJSONObject(plan.ConfigSensitive, "config_sensitive", diags),
		priorSensitive,
	)

	if state == nil {
		// Create: optional fields are simply left out.
		body["type"] = plan.Type.ValueString()

		putString(body, "description", plan.Description)
		putString(body, "checkGroupUid", plan.CheckGroupUID)
		putString(body, "escalationPolicyUid", plan.EscalationPolicyUID)

		if len(labels) > 0 {
			body["labels"] = labels
		}

		return body
	}

	// Update: an empty value clears the field.
	body["description"] = stringOrEmpty(plan.Description)
	body["checkGroupUid"] = stringOrEmpty(plan.CheckGroupUID)
	body["escalationPolicyUid"] = stringOrEmpty(plan.EscalationPolicyUID)
	body["labels"] = labels

	return body
}

// apply copies the server answer into the model. The JSON attributes are left
// to the caller: they are kept as planned after a write, refreshed on read.
func (m *checkModel) apply(ctx context.Context, check *client.Check, diags *diag.Diagnostics) {
	m.ID = types.StringValue(check.UID)
	m.Name = types.StringValue(check.Name)
	m.Slug = types.StringValue(check.Slug)
	m.Description = optionalString(deref(check.Description), m.Description)
	m.Type = types.StringValue(check.Type)
	m.Enabled = types.BoolValue(check.Enabled)
	m.Period = types.StringValue(check.Period)
	m.Placement = types.StringValue(check.Placement)
	m.CheckGroupUID = optionalString(deref(check.CheckGroupUID), m.CheckGroupUID)
	m.EscalationPolicyUID = optionalString(deref(check.EscalationPolicyUID), m.EscalationPolicyUID)

	if check.RegionCount != nil {
		m.RegionCount = types.Int64Value(*check.RegionCount)
	} else {
		m.RegionCount = types.Int64Null()
	}

	regions := check.Regions
	if regions == nil {
		regions = []string{}
	}

	list, d := types.ListValueFrom(ctx, types.StringType, regions)
	diags.Append(d...)

	m.Regions = list

	if len(check.Labels) == 0 && (m.Labels.IsNull() || m.Labels.IsUnknown()) {
		m.Labels = types.MapNull(types.StringType)
	} else {
		labels := check.Labels
		if labels == nil {
			labels = map[string]string{}
		}

		mapValue, d := types.MapValueFrom(ctx, types.StringType, labels)
		diags.Append(d...)

		m.Labels = mapValue
	}
}

func (r *checkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config checkModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body := r.payload(ctx, &plan, &config, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	check, err := r.api.CreateCheck(ctx, body)
	if err != nil {
		addAPIError(&resp.Diagnostics, "create the check", err)

		return
	}

	plan.apply(ctx, check, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *checkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state checkModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	check, err := r.api.GetCheck(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		addAPIError(&resp.Diagnostics, "read the check", err)

		return
	}

	state.apply(ctx, check, &resp.Diagnostics)
	state.Config = refreshJSONAttr(state.Config, "config", check.Config, check.ConfigPrivateKeys, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *checkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config, state checkModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body := r.payload(ctx, &plan, &config, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	check, err := r.api.UpdateCheck(ctx, state.ID.ValueString(), body)
	if err != nil {
		addAPIError(&resp.Diagnostics, "update the check", err)

		return
	}

	plan.apply(ctx, check, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *checkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state checkModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.api.DeleteCheck(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		addAPIError(&resp.Diagnostics, "delete the check", err)
	}
}

func (r *checkResource) ImportState(
	ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	importByOrgUID(ctx, r.api, req, resp)
}
