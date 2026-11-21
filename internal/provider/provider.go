// Package provider implements the SolidPing Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

const (
	envEndpoint = "SOLIDPING_ENDPOINT"
	envOrg      = "SOLIDPING_ORG"
	envToken    = "SOLIDPING_TOKEN" //nolint:gosec // name of an environment variable, not a credential
)

var _ provider.Provider = (*solidpingProvider)(nil)

type solidpingProvider struct {
	version string
}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Org      types.String `tfsdk:"org"`
	Token    types.String `tfsdk:"token"`
}

// New returns the provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &solidpingProvider{version: version}
	}
}

func (p *solidpingProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "solidping"
	resp.Version = p.version
}

func (p *solidpingProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage [SolidPing](https://www.solidping.io) monitoring as code: checks, " +
			"notification channels, status pages and maintenance windows.\n\n" +
			"The provider talks to one organization of one SolidPing server, with an organization-scoped " +
			"personal access token (dashboard: **Account → Tokens**, or `POST /api/v1/orgs/{org}/tokens`).",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Base URL of the SolidPing server, e.g. `https://solidping.acme.com`. " +
					"Falls back to the `" + envEndpoint + "` environment variable.",
			},
			"org": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Slug of the organization to manage. " +
					"Falls back to the `" + envOrg + "` environment variable.",
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Organization-scoped personal access token. " +
					"Falls back to the `" + envToken + "` environment variable.",
			},
		},
	}
}

func (p *solidpingProvider) Configure(
	ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse,
) {
	var cfg providerModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)

	if resp.Diagnostics.HasError() {
		return
	}

	settings := []struct {
		attr  string
		env   string
		value types.String
		out   string
	}{
		{attr: "endpoint", env: envEndpoint, value: cfg.Endpoint},
		{attr: "org", env: envOrg, value: cfg.Org},
		{attr: "token", env: envToken, value: cfg.Token},
	}

	for i := range settings {
		s := &settings[i]

		if s.value.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root(s.attr), "Unknown provider setting",
				"`"+s.attr+"` is not known yet. Set it to a static value or use the `"+s.env+"` environment variable.")

			continue
		}

		s.out = os.Getenv(s.env)
		if !s.value.IsNull() {
			s.out = s.value.ValueString()
		}

		if s.out == "" {
			resp.Diagnostics.AddAttributeError(path.Root(s.attr), "Missing provider setting",
				"Set `"+s.attr+"` in the provider block or the `"+s.env+"` environment variable.")
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	api := client.New(settings[0].out, settings[1].out, settings[2].out, "terraform-provider-solidping/"+p.version)

	resp.ResourceData = api
	resp.DataSourceData = api
}

func (p *solidpingProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newCheckResource,
		newChannelResource,
		newStatusPageResource,
		newStatusPageSectionResource,
		newMaintenanceWindowResource,
	}
}

func (p *solidpingProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newCheckDataSource,
		newChannelDataSource,
		newStatusPageDataSource,
	}
}
