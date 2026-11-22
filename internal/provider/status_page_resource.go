package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

var (
	_ resource.Resource                = (*statusPageResource)(nil)
	_ resource.ResourceWithConfigure   = (*statusPageResource)(nil)
	_ resource.ResourceWithImportState = (*statusPageResource)(nil)
)

type statusPageResource struct {
	api *client.Client
}

type statusPageModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Slug                    types.String `tfsdk:"slug"`
	Description             types.String `tfsdk:"description"`
	Visibility              types.String `tfsdk:"visibility"`
	Password                types.String `tfsdk:"password"`
	Enabled                 types.Bool   `tfsdk:"enabled"`
	IsDefault               types.Bool   `tfsdk:"is_default"`
	ShowAvailability        types.Bool   `tfsdk:"show_availability"`
	ShowResponseTime        types.Bool   `tfsdk:"show_response_time"`
	HistoryPeriod           types.String `tfsdk:"history_period"`
	Language                types.String `tfsdk:"language"`
	AutoPublish             types.Bool   `tfsdk:"auto_publish"`
	AutoPublishDelaySeconds types.Int64  `tfsdk:"auto_publish_delay_seconds"`
	PublishDegraded         types.Bool   `tfsdk:"publish_degraded"`
	AutoResolve             types.String `tfsdk:"auto_resolve"`
	HideBranding            types.Bool   `tfsdk:"hide_branding"`
	CustomCSS               types.String `tfsdk:"custom_css"`
	CustomDomain            types.String `tfsdk:"custom_domain"`
}

func newStatusPageResource() resource.Resource { return &statusPageResource{} }

func (r *statusPageResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_status_page"
}

func serverDefaultBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
	}
}

func (r *statusPageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A status page. This resource manages the page itself; its content is declared with " +
			"`solidping_status_page_section`.\n\n" +
			"-> SolidPing seeds every new page with an empty \"Services\" section. The provider removes it " +
			"right after creating the page, so that a page managed with Terraform holds only the sections " +
			"declared with `solidping_status_page_section`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UID of the status page.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Title of the page.",
			},
			"slug": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "URL-friendly identifier, unique in the organization. Derived from `name` when omitted.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Text shown under the title.",
			},
			"visibility": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "`public` (world-readable, the default), `private` (hidden) or `password` " +
					"(unlocked with `password`).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:    []validator.String{stringvalidator.OneOf("public", "private", "password")},
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Unlock password, required when `visibility` is `password` (6 characters minimum). " +
					"Never read back from the server.",
				Validators: []validator.String{stringvalidator.LengthAtLeast(6)},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the page is served. Defaults to `true`.",
			},
			"is_default":         serverDefaultBool("Whether this is the default status page of the organization."),
			"show_availability":  serverDefaultBool("Show the availability bars. Defaults to `true`."),
			"show_response_time": serverDefaultBool("Show the response-time charts. Defaults to `true`."),
			"history_period": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "History window: `24h`, `7d`, `30d` or `90d` (the default).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.OneOf("24h", "7d", "30d", "90d")},
			},
			"language": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Language of the page (e.g. `en`, `fr`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"auto_publish": serverDefaultBool(
				"Publish incidents affecting the page's resources automatically. Defaults to `true`."),
			"auto_publish_delay_seconds": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Delay before an incident becomes public, in seconds (0 to 86400, default 60). " +
					"An incident that resolves within this delay is never published.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Validators:    []validator.Int64{int64validator.Between(0, 86400)},
			},
			"publish_degraded": serverDefaultBool("Also publish degraded incidents automatically. Defaults to `false`."),
			"auto_resolve": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "What an auto-published incident does when it resolves: `always`, " +
					"`if_untouched` (the default) or `never`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:    []validator.String{stringvalidator.OneOf("always", "if_untouched", "never")},
			},
			"hide_branding": serverDefaultBool(
				"Hide the SolidPing branding (applies when the organization has the white-label entitlement)."),
			"custom_css": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Custom stylesheet injected in the public page (64 KB maximum, no `@import`).",
			},
			"custom_domain": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Hostname the page is served on, e.g. `status.acme.com`. " +
					"The domain still has to be verified (DNS) in SolidPing.",
			},
		},
	}
}

func (r *statusPageResource) Configure(
	_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse,
) {
	r.api = configureClient(req.ProviderData, &resp.Diagnostics)
}

// payload builds the create (state == nil) or update body.
func (r *statusPageResource) payload(plan, state *statusPageModel) client.Payload {
	body := client.Payload{"name": plan.Name.ValueString()}

	putString(body, "slug", plan.Slug)
	putString(body, "visibility", plan.Visibility)
	putBool(body, "isDefault", plan.IsDefault)
	putBool(body, "showAvailability", plan.ShowAvailability)
	putBool(body, "showResponseTime", plan.ShowResponseTime)
	putString(body, "historyPeriod", plan.HistoryPeriod)
	putString(body, "language", plan.Language)
	putBool(body, "autoPublish", plan.AutoPublish)
	putInt(body, "autoPublishDelaySeconds", plan.AutoPublishDelaySeconds)
	putBool(body, "publishDegraded", plan.PublishDegraded)
	putString(body, "autoResolve", plan.AutoResolve)
	putBool(body, "hideBranding", plan.HideBranding)

	if state == nil {
		putString(body, "description", plan.Description)
		putString(body, "password", plan.Password)
		putString(body, "customCss", plan.CustomCSS)
		putString(body, "customDomain", plan.CustomDomain)

		return body
	}

	// `enabled` can only be written on update.
	putBool(body, "enabled", plan.Enabled)

	body["description"] = stringOrEmpty(plan.Description)
	body["customCss"] = stringOrEmpty(plan.CustomCSS)

	if !plan.Password.Equal(state.Password) {
		body["password"] = stringOrEmpty(plan.Password)
	}

	if !plan.CustomDomain.Equal(state.CustomDomain) {
		if plan.CustomDomain.IsNull() {
			body["customDomain"] = nil
		} else {
			body["customDomain"] = plan.CustomDomain.ValueString()
		}
	}

	return body
}

func (m *statusPageModel) apply(page *client.StatusPage) {
	m.ID = types.StringValue(page.UID)
	m.Name = types.StringValue(page.Name)
	m.Slug = types.StringValue(page.Slug)
	m.Description = optionalString(page.Description, m.Description)
	m.Visibility = types.StringValue(page.Visibility)
	m.Enabled = types.BoolValue(page.Enabled)
	m.IsDefault = types.BoolValue(page.IsDefault)
	m.ShowAvailability = types.BoolValue(page.ShowAvailability)
	m.ShowResponseTime = types.BoolValue(page.ShowResponseTime)
	m.HistoryPeriod = types.StringValue(page.HistoryPeriod)
	m.Language = types.StringValue(page.Language)
	m.AutoPublish = types.BoolValue(page.AutoPublish)
	m.AutoPublishDelaySeconds = types.Int64Value(page.AutoPublishDelaySeconds)
	m.PublishDegraded = types.BoolValue(page.PublishDegraded)
	m.AutoResolve = types.StringValue(page.AutoResolve)
	m.HideBranding = types.BoolValue(page.HideBranding)
	m.CustomCSS = optionalString(deref(page.CustomCSS), m.CustomCSS)
	m.CustomDomain = optionalString(deref(page.CustomDomain), m.CustomDomain)
}

func (r *statusPageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan statusPageModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	page, err := r.api.CreateStatusPage(ctx, r.payload(&plan, nil))
	if err != nil {
		addAPIError(&resp.Diagnostics, "create the status page", err)

		return
	}

	wantEnabled := plan.Enabled

	plan.apply(page)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	r.removeSeededSections(ctx, page.UID, resp)

	// A page is created enabled; disabling it takes a second call.
	if !wantEnabled.IsNull() && !wantEnabled.IsUnknown() && wantEnabled.ValueBool() != page.Enabled {
		page, err = r.api.UpdateStatusPage(ctx, page.UID, client.Payload{"enabled": wantEnabled.ValueBool()})
		if err != nil {
			addAPIError(&resp.Diagnostics, "set `enabled` on the new status page", err)

			return
		}

		plan.apply(page)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

// removeSeededSections deletes the sections the server seeds a new page with.
func (r *statusPageResource) removeSeededSections(ctx context.Context, pageUID string, resp *resource.CreateResponse) {
	sections, err := r.api.ListStatusPageSections(ctx, pageUID)
	if err != nil {
		resp.Diagnostics.AddWarning("Default section not removed",
			"The status page was created, but its seeded sections could not be listed: "+err.Error())

		return
	}

	for i := range sections {
		if err := r.api.DeleteStatusPageSection(ctx, pageUID, sections[i].UID); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddWarning("Default section not removed",
				"The status page was created, but its seeded section `"+sections[i].Name+
					"` could not be removed: "+err.Error())
		}
	}
}

func (r *statusPageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state statusPageModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	page, err := r.api.GetStatusPage(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		addAPIError(&resp.Diagnostics, "read the status page", err)

		return
	}

	state.apply(page)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *statusPageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state statusPageModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	page, err := r.api.UpdateStatusPage(ctx, state.ID.ValueString(), r.payload(&plan, &state))
	if err != nil {
		addAPIError(&resp.Diagnostics, "update the status page", err)

		return
	}

	plan.apply(page)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *statusPageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state statusPageModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.api.DeleteStatusPage(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		addAPIError(&resp.Diagnostics, "delete the status page", err)
	}
}

func (r *statusPageResource) ImportState(
	ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	importByOrgUID(ctx, r.api, req, resp)
}
