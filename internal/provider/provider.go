// Package provider implements the Silo OpenTofu provider on
// terraform-plugin-framework (protocol 6). One file per resource or data
// source; this file holds the provider definition and registries.
package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providercfg "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/barolinecrewer/silo-server-tofu/internal/client"
)

// providerTypeName is the prefix for every resource and data source
// (silo_access_group, ...).
const providerTypeName = "silo"

// Env var fallbacks for provider config. Useful in CI and for testacc.
const (
	envBaseURL = "SILO_BASE_URL"
	envAPIKey  = "SILO_API_KEY"
)

// New returns a provider factory bound to a version string.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &siloProvider{version: version}
	}
}

type siloProvider struct {
	version string
}

// siloProviderModel is the provider configuration schema.
type siloProviderModel struct {
	BaseURL types.String `tfsdk:"base_url"`
	APIKey  types.String `tfsdk:"api_key"`
}

// Metadata sets the provider type name.
func (p *siloProvider) Metadata(_ context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = providerTypeName
	resp.Version = p.version
}

// Schema defines the provider configuration.
func (p *siloProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providercfg.Schema{
		Description: "Configure a Silo media server (https://github.com/Silo-Server) " +
			"through its /api/v2 API: accounts, access groups, libraries, and other " +
			"administrative configuration as infrastructure as code.",
		Attributes: map[string]providercfg.Attribute{
			"base_url": providercfg.StringAttribute{
				Required: true,
				Description: "Silo server root, for example \"https://silo.example.org\". " +
					"Must not include the /api/v2 path. " +
					"Defaults to the " + envBaseURL + " environment variable.",
			},
			"api_key": providercfg.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "API key for the server, sent as a bearer token. Keys inherit " +
					"their owner's permissions: admin resources need an admin-owned key. " +
					"Defaults to the " + envAPIKey + " environment variable.",
			},
		},
	}
}

// Configure builds the shared API client from config (with environment
// fallbacks) and hands it to every resource and data source.
func (p *siloProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config siloProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv(envBaseURL)
	}
	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv(envAPIKey)
	}

	cl, err := client.New(client.Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Silo provider configuration",
			fmt.Sprintf("The provider could not build an API client: %s. "+
				"Set base_url to the server root (without /api/v2) and api_key to a valid key, "+
				"or set the %s and %s environment variables.",
				err, envBaseURL, envAPIKey),
		)
		return
	}

	resp.ResourceData = cl
	resp.DataSourceData = cl
}

// Resources registers every managed resource.
func (p *siloProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAccessGroupResource,
	}
}

// DataSources registers every data source.
func (p *siloProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAccessGroupsDataSource,
	}
}

// Ensure the provider satisfies the framework interface at compile time.
var _ provider.Provider = (*siloProvider)(nil)
