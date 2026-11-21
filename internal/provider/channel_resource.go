package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

var (
	_ resource.Resource                = (*channelResource)(nil)
	_ resource.ResourceWithConfigure   = (*channelResource)(nil)
	_ resource.ResourceWithImportState = (*channelResource)(nil)
)

type channelResource struct {
	api *client.Client
}

type channelModel struct {
	ID                types.String         `tfsdk:"id"`
	Type              types.String         `tfsdk:"type"`
	Name              types.String         `tfsdk:"name"`
	Enabled           types.Bool           `tfsdk:"enabled"`
	IsDefault         types.Bool           `tfsdk:"is_default"`
	Settings          jsontypes.Normalized `tfsdk:"settings"`
	SettingsSensitive jsontypes.Normalized `tfsdk:"settings_sensitive"`
}

func newChannelResource() resource.Resource { return &channelResource{} }

func (r *channelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_channel"
}

func (r *channelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A notification channel: webhook, email, Discord, Google Chat, Mattermost, ntfy, " +
			"PagerDuty, Pushover, ...\n\n" +
			"~> **Slack channels cannot be managed with Terraform.** They are created by installing the " +
			"SolidPing Slack app (an OAuth flow), and the API refuses to create one directly. " +
			"Reference an existing one with the `solidping_channel` data source.\n\n" +
			"The type-specific settings go in `settings`, as a JSON object. Secrets go in " +
			"`settings_sensitive`: the API stores them encrypted and never returns them.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UID of the channel.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Channel type: `webhook`, `email`, `discord`, `googlechat`, `mattermost`, `ntfy`, " +
					"`pagerduty`, `pushover`, ... Changing it replaces the channel.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.NoneOf("slack"),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether notifications are sent to this channel. Defaults to `true`.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"is_default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether new checks notify this channel by default.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"settings": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
				MarkdownDescription: "Type-specific settings, as a JSON object: `jsonencode({ url = \"https://acme.com/hook\" })`. " +
					"Only the keys written here are compared with the server.",
			},
			"settings_sensitive": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
				Sensitive:  true,
				MarkdownDescription: "Secret settings, as a JSON object merged over `settings` when the channel is " +
					"written. Never read back from the server. Removing a key clears the stored secret.",
			},
		},
	}
}

func (r *channelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.api = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *channelResource) payload(plan, state *channelModel, diags *diag.Diagnostics) client.Payload {
	body := client.Payload{"name": plan.Name.ValueString()}

	putBool(body, "enabled", plan.Enabled)
	putBool(body, "isDefault", plan.IsDefault)

	var priorSensitive map[string]any

	if state == nil {
		body["type"] = plan.Type.ValueString()
	} else {
		priorSensitive = decodeJSONObject(state.SettingsSensitive, "settings_sensitive", diags)
	}

	// The settings object is replaced as a whole on update; secret keys left
	// out of it are preserved by the server.
	body["settings"] = mergeSecrets(
		decodeJSONObject(plan.Settings, "settings", diags),
		decodeJSONObject(plan.SettingsSensitive, "settings_sensitive", diags),
		priorSensitive,
	)

	return body
}

func (m *channelModel) apply(channel *client.Channel) {
	m.ID = types.StringValue(channel.UID)
	m.Type = types.StringValue(channel.Type)
	m.Name = types.StringValue(channel.Name)
	m.Enabled = types.BoolValue(channel.Enabled)
	m.IsDefault = types.BoolValue(channel.IsDefault)
}

func (r *channelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan channelModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body := r.payload(&plan, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	channel, err := r.api.CreateChannel(ctx, body)
	if err != nil {
		addAPIError(&resp.Diagnostics, "create the channel", err)

		return
	}

	plan.apply(channel)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *channelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state channelModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	channel, err := r.api.GetChannel(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		addAPIError(&resp.Diagnostics, "read the channel", err)

		return
	}

	// A read by UID returns the generated webhook signing secret in clear,
	// among the public settings: it is not configuration, keep it out of an
	// imported state.
	privateKeys := append([]string{"signingSecret"}, channel.SettingsPrivateKeys...)

	state.apply(channel)
	state.Settings = refreshJSONAttr(state.Settings, "settings", channel.Settings, privateKeys, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *channelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state channelModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body := r.payload(&plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	channel, err := r.api.UpdateChannel(ctx, state.ID.ValueString(), body)
	if err != nil {
		addAPIError(&resp.Diagnostics, "update the channel", err)

		return
	}

	plan.apply(channel)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *channelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state channelModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.api.DeleteChannel(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		addAPIError(&resp.Diagnostics, "delete the channel", err)
	}
}

func (r *channelResource) ImportState(
	ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	importByOrgUID(ctx, r.api, req, resp)
}
