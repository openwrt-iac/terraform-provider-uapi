package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ provider.Provider                       = &uapiProvider{}
	_ provider.ProviderWithEphemeralResources = &uapiProvider{}
)

type uapiProvider struct {
	version string
}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
	Insecure types.Bool   `tfsdk:"insecure"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &uapiProvider{version: version}
	}
}

func (p *uapiProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "uapi"
	resp.Version = p.version
}

func (p *uapiProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage OpenWrt configuration through the uapi REST API. " +
			"Only curated endpoints are exposed; the /raw passthrough is intentionally not supported. " +
			"See the [uapi project](https://github.com/openwrt-iac/uapi) for the API this provider wraps.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional: true,
				Description: "Base URL of the uapi API, including the version prefix, " +
					"e.g. https://router.example.com/api/v1. May also be set via the " +
					"UAPI_ENDPOINT or UAPI_BASE environment variable.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Bearer token created with `uapi-token create`. May also be set via the UAPI_TOKEN environment variable.",
			},
			"insecure": schema.BoolAttribute{
				Optional: true,
				Description: "Skip TLS certificate verification. Needed for uapi's default self-signed " +
					"certificate; do not use in production. May also be set via UAPI_INSECURE.",
			},
		},
	}
}

func (p *uapiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := firstNonEmpty(cfg.Endpoint, os.Getenv("UAPI_ENDPOINT"), os.Getenv("UAPI_BASE"))
	token := firstNonEmpty(cfg.Token, os.Getenv("UAPI_TOKEN"))

	insecure := false
	if !cfg.Insecure.IsNull() && !cfg.Insecure.IsUnknown() {
		insecure = cfg.Insecure.ValueBool()
	} else if v := os.Getenv("UAPI_INSECURE"); v == "1" || v == "true" {
		insecure = true
	}

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"),
			"Missing uapi endpoint",
			"Set the provider `endpoint` argument or the UAPI_ENDPOINT environment variable.")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(path.Root("token"),
			"Missing uapi token",
			"Set the provider `token` argument or the UAPI_TOKEN environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c := client.New(endpoint, token, insecure, p.version)
	resp.ResourceData = c
	resp.DataSourceData = c
	resp.EphemeralResourceData = c
}

func (p *uapiProvider) Resources(_ context.Context) []func() resource.Resource {
	// Generated curated resources (internal/gen) plus hand-written specials.
	return append(generatedResources(),
		NewPackageResource,
		NewPackageFeedResource,
		NewAuthorizedKeyResource,
		NewSystemPasswordResource,
	)
}

func (p *uapiProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return append(generatedDataSources(),
		NewDHCPLeasesDataSource,
		NewDHCPLeases6DataSource,
		NewAuthorizedKeyDataSource,
		NewPackageDataSource,
		NewPackageFeedDataSource,
		NewWhoamiDataSource,
		NewHealthzDataSource,
		NewDiagnosticsDataSource,
	)
}

func (p *uapiProvider) EphemeralResources(_ context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewTokenEphemeral,
	}
}

func firstNonEmpty(configured types.String, fallbacks ...string) string {
	if !configured.IsNull() && !configured.IsUnknown() && configured.ValueString() != "" {
		return configured.ValueString()
	}
	for _, f := range fallbacks {
		if f != "" {
			return f
		}
	}
	return ""
}
