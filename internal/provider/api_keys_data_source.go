package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/barolinecrewer/silo-server-tofu/internal/client"
)

// Ensure the data source satisfies the framework interfaces at compile time.
var (
	_ datasource.DataSource              = (*apiKeysDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*apiKeysDataSource)(nil)
)

// NewAPIKeysDataSource is the factory registered in provider.go.
func NewAPIKeysDataSource() datasource.DataSource {
	return &apiKeysDataSource{}
}

// apiKeysDataSource reads every API key on the server, walking the API's
// cursor pagination. It never exposes credential values: the list endpoint
// returns only each key's non-secret prefix.
type apiKeysDataSource struct {
	client *client.Client
}

// apiKeysDataSourceModel is the data source state: one attribute holding
// every key.
type apiKeysDataSourceModel struct {
	APIKeys types.List `tfsdk:"api_keys"`
}

// apiKeyListItemModel is one element of the api_keys list.
type apiKeyListItemModel struct {
	ID         types.String `tfsdk:"id"`
	UserID     types.String `tfsdk:"user_id"`
	Username   types.String `tfsdk:"username"`
	Label      types.String `tfsdk:"label"`
	KeyPrefix  types.String `tfsdk:"key_prefix"`
	RateTier   types.String `tfsdk:"rate_tier"`
	Scopes     types.Set    `tfsdk:"scopes"`
	LastUsedAt types.String `tfsdk:"last_used_at"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

// apiKeyListItemObjectType mirrors apiKeyListItemModel's tfsdk tags; keep the
// two in sync when adding fields.
var apiKeyListItemObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":           types.StringType,
		"user_id":      types.StringType,
		"username":     types.StringType,
		"label":        types.StringType,
		"key_prefix":   types.StringType,
		"rate_tier":    types.StringType,
		"scopes":       types.SetType{ElemType: types.StringType},
		"last_used_at": types.StringType,
		"created_at":   types.StringType,
	},
}

// Metadata sets the data source type name.
func (d *apiKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_keys"
}

// Schema defines the data source schema.
func (d *apiKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "All API keys on the server. Credential values are never " +
			"returned; each key exposes only its non-secret prefix. Use to audit " +
			"keys, their owners, and their rate tiers.",
		Attributes: map[string]schema.Attribute{
			"api_keys": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every API key on the server.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Opaque Silo API key identifier.",
						},
						"user_id": schema.StringAttribute{
							Computed:    true,
							Description: "Opaque ID of the account owning the key.",
						},
						"username": schema.StringAttribute{
							Computed:    true,
							Description: "Username of the account owning the key.",
						},
						"label": schema.StringAttribute{
							Computed:    true,
							Description: "Display label of the key.",
						},
						"key_prefix": schema.StringAttribute{
							Computed:    true,
							Description: "Non-secret prefix of the credential.",
						},
						"rate_tier": schema.StringAttribute{
							Computed:    true,
							Description: "Rate limit tier: standard or elevated.",
						},
						"scopes": schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Scopes granted to the key.",
						},
						"last_used_at": schema.StringAttribute{
							Computed:    true,
							Description: "RFC 3339 last-use timestamp, if the key has been used.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "RFC 3339 creation timestamp.",
						},
					},
				},
			},
		},
	}
}

// Configure receives the shared API client from the provider.
func (d *apiKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cl, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got: %T. Report this as a provider bug.", req.ProviderData),
		)
		return
	}
	d.client = cl
}

// Read walks every page of the API key collection into state.
func (d *apiKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	keys, err := d.client.ListAllAPIKeys(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing api keys",
			"Could not list api keys: "+err.Error(),
		)
		return
	}

	elements := make([]apiKeyListItemModel, 0, len(keys))
	for i := range keys {
		var lastUsedAt types.String
		if keys[i].LastUsedAt == nil {
			lastUsedAt = types.StringNull()
		} else {
			lastUsedAt = types.StringValue(*keys[i].LastUsedAt)
		}
		elements = append(elements, apiKeyListItemModel{
			ID:         types.StringValue(keys[i].ID),
			UserID:     types.StringValue(keys[i].UserID),
			Username:   types.StringValue(keys[i].Username),
			Label:      types.StringValue(keys[i].Label),
			KeyPrefix:  types.StringValue(keys[i].KeyPrefix),
			RateTier:   types.StringValue(keys[i].RateTier),
			Scopes:     stringSetFromSlice(ctx, keys[i].Scopes),
			LastUsedAt: lastUsedAt,
			CreatedAt:  types.StringValue(keys[i].CreatedAt),
		})
	}

	list, diags := types.ListValueFrom(ctx, apiKeyListItemObjectType, elements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := apiKeysDataSourceModel{APIKeys: list}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
