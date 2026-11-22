package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

var (
	_ datasource.DataSourceWithConfigure = (*checkDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*channelDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*statusPageDataSource)(nil)
)

// lookupAttribute is one of the two attributes a data source is looked up by.
func lookupAttribute(description, other string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: description,
		Validators: []validator.String{
			stringvalidator.ExactlyOneOf(path.MatchRoot(other)),
		},
	}
}

func computedString(description string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, MarkdownDescription: description}
}

func computedBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{Computed: true, MarkdownDescription: description}
}

// --- solidping_check ---

type checkDataSource struct {
	api *client.Client
}

type checkDataModel struct {
	ID          types.String         `tfsdk:"id"`
	UID         types.String         `tfsdk:"uid"`
	Slug        types.String         `tfsdk:"slug"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	Type        types.String         `tfsdk:"type"`
	Config      jsontypes.Normalized `tfsdk:"config"`
	Enabled     types.Bool           `tfsdk:"enabled"`
	Period      types.String         `tfsdk:"period"`
	Regions     types.List           `tfsdk:"regions"`
	Labels      types.Map            `tfsdk:"labels"`
	Status      types.String         `tfsdk:"status"`
}

func newCheckDataSource() datasource.DataSource { return &checkDataSource{} }

func (d *checkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

func (d *checkDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing check by `uid` or by `slug`.",
		Attributes: map[string]schema.Attribute{
			"id":          computedString("UID of the check."),
			"uid":         lookupAttribute("UID of the check. Exactly one of `uid` and `slug` must be set.", "slug"),
			"slug":        lookupAttribute("Slug of the check. Exactly one of `uid` and `slug` must be set.", "uid"),
			"name":        computedString("Display name."),
			"description": computedString("What this check monitors."),
			"type":        computedString("Check type (`http`, `tcp`, ...)."),
			"config": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				Computed:            true,
				MarkdownDescription: "Type-specific configuration, as a JSON object (secrets excluded). Read it with `jsondecode`.",
			},
			"enabled": computedBool("Whether the check runs."),
			"period":  computedString("Interval between two runs, as `HH:MM:SS`."),
			"regions": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Region slugs the check runs from.",
			},
			"labels": schema.MapAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Key-value labels.",
			},
			"status": computedString("Current status of the check (`up`, `down`, `degraded`, ...)."),
		},
	}
}

func (d *checkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.api = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *checkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data checkDataModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	key := data.UID.ValueString()
	if key == "" {
		key = data.Slug.ValueString()
	}

	check, err := d.api.GetCheck(ctx, key)
	if err != nil {
		addAPIError(&resp.Diagnostics, "read the check `"+key+"`", err)

		return
	}

	config := check.Config
	if config == nil {
		config = map[string]any{}
	}

	regions, diags := types.ListValueFrom(ctx, types.StringType, append([]string{}, check.Regions...))
	resp.Diagnostics.Append(diags...)

	labels := check.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	labelsValue, diags := types.MapValueFrom(ctx, types.StringType, labels)
	resp.Diagnostics.Append(diags...)

	data = checkDataModel{
		ID:          types.StringValue(check.UID),
		UID:         types.StringValue(check.UID),
		Slug:        types.StringValue(check.Slug),
		Name:        types.StringValue(check.Name),
		Description: types.StringValue(deref(check.Description)),
		Type:        types.StringValue(check.Type),
		Config:      encodeJSONObject(config, &resp.Diagnostics),
		Enabled:     types.BoolValue(check.Enabled),
		Period:      types.StringValue(check.Period),
		Regions:     regions,
		Labels:      labelsValue,
		Status:      types.StringValue(check.Status),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// --- solidping_channel ---

type channelDataSource struct {
	api *client.Client
}

type channelDataModel struct {
	ID        types.String         `tfsdk:"id"`
	UID       types.String         `tfsdk:"uid"`
	Name      types.String         `tfsdk:"name"`
	Type      types.String         `tfsdk:"type"`
	Enabled   types.Bool           `tfsdk:"enabled"`
	IsDefault types.Bool           `tfsdk:"is_default"`
	Settings  jsontypes.Normalized `tfsdk:"settings"`
}

func newChannelDataSource() datasource.DataSource { return &channelDataSource{} }

func (d *channelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_channel"
}

func (d *channelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing notification channel by `uid` or by `name`. " +
			"This is how a Slack channel, which Terraform cannot create, is referenced.",
		Attributes: map[string]schema.Attribute{
			"id":  computedString("UID of the channel."),
			"uid": lookupAttribute("UID of the channel. Exactly one of `uid` and `name` must be set.", "name"),
			"name": lookupAttribute("Name of the channel. Exactly one of `uid` and `name` must be set. "+
				"The lookup fails when several channels share the name.", "uid"),
			"type":       computedString("Channel type (`slack`, `webhook`, `email`, ...)."),
			"enabled":    computedBool("Whether notifications are sent to this channel."),
			"is_default": computedBool("Whether new checks notify this channel by default."),
			"settings": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				Computed:            true,
				MarkdownDescription: "Type-specific settings, as a JSON object (secrets excluded). Read it with `jsondecode`.",
			},
		},
	}
}

func (d *channelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.api = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *channelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data channelDataModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	uid := data.UID.ValueString()

	if uid == "" {
		name := data.Name.ValueString()

		channels, err := d.api.ListChannels(ctx)
		if err != nil {
			addAPIError(&resp.Diagnostics, "list the channels", err)

			return
		}

		var matches []string

		for i := range channels {
			if channels[i].Name == name {
				matches = append(matches, channels[i].UID)
			}
		}

		if len(matches) != 1 {
			resp.Diagnostics.AddAttributeError(path.Root("name"), "Channel lookup failed",
				"Expected exactly one channel named `"+name+"`, found "+strconv.Itoa(len(matches))+". Use `uid` instead.")

			return
		}

		uid = matches[0]
	}

	// The list omits the settings: always read the channel itself.
	channel, err := d.api.GetChannel(ctx, uid)
	if err != nil {
		addAPIError(&resp.Diagnostics, "read the channel `"+uid+"`", err)

		return
	}

	settings := channel.Settings
	if settings == nil {
		settings = map[string]any{}
	}

	data = channelDataModel{
		ID:        types.StringValue(channel.UID),
		UID:       types.StringValue(channel.UID),
		Name:      types.StringValue(channel.Name),
		Type:      types.StringValue(channel.Type),
		Enabled:   types.BoolValue(channel.Enabled),
		IsDefault: types.BoolValue(channel.IsDefault),
		Settings:  encodeJSONObject(settings, &resp.Diagnostics),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// --- solidping_status_page ---

type statusPageDataSource struct {
	api *client.Client
}

type statusPageDataModel struct {
	ID           types.String `tfsdk:"id"`
	UID          types.String `tfsdk:"uid"`
	Slug         types.String `tfsdk:"slug"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Visibility   types.String `tfsdk:"visibility"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	IsDefault    types.Bool   `tfsdk:"is_default"`
	CustomDomain types.String `tfsdk:"custom_domain"`
}

func newStatusPageDataSource() datasource.DataSource { return &statusPageDataSource{} }

func (d *statusPageDataSource) Metadata(
	_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_status_page"
}

func (d *statusPageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing status page by `uid` or by `slug`.",
		Attributes: map[string]schema.Attribute{
			"id":            computedString("UID of the status page."),
			"uid":           lookupAttribute("UID of the status page. Exactly one of `uid` and `slug` must be set.", "slug"),
			"slug":          lookupAttribute("Slug of the status page. Exactly one of `uid` and `slug` must be set.", "uid"),
			"name":          computedString("Title of the page."),
			"description":   computedString("Text shown under the title."),
			"visibility":    computedString("`public`, `private` or `password`."),
			"enabled":       computedBool("Whether the page is served."),
			"is_default":    computedBool("Whether this is the default status page of the organization."),
			"custom_domain": computedString("Hostname the page is served on, empty when none."),
		},
	}
}

func (d *statusPageDataSource) Configure(
	_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse,
) {
	d.api = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *statusPageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data statusPageDataModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	key := data.UID.ValueString()
	if key == "" {
		key = data.Slug.ValueString()
	}

	page, err := d.api.GetStatusPage(ctx, key)
	if err != nil {
		addAPIError(&resp.Diagnostics, "read the status page `"+key+"`", err)

		return
	}

	data = statusPageDataModel{
		ID:           types.StringValue(page.UID),
		UID:          types.StringValue(page.UID),
		Slug:         types.StringValue(page.Slug),
		Name:         types.StringValue(page.Name),
		Description:  types.StringValue(page.Description),
		Visibility:   types.StringValue(page.Visibility),
		Enabled:      types.BoolValue(page.Enabled),
		IsDefault:    types.BoolValue(page.IsDefault),
		CustomDomain: types.StringValue(deref(page.CustomDomain)),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
