package provider

import (
	"context"
	"fmt"

	"github.com/barolinecrewer/silo-server-tofu/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*accountsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*accountsDataSource)(nil)

func NewAccountsDataSource() datasource.DataSource { return &accountsDataSource{} }

type accountsDataSource struct{ client *client.Client }
type accountsDataSourceModel struct {
	Accounts types.List `tfsdk:"accounts"`
}
type accountListItemModel struct {
	ID            types.String `tfsdk:"id"`
	Username      types.String `tfsdk:"username"`
	Email         types.String `tfsdk:"email"`
	Role          types.String `tfsdk:"role"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	Permissions   types.Set    `tfsdk:"permissions"`
	AccessGroupID types.String `tfsdk:"access_group_id"`
	LibraryIDs    types.Set    `tfsdk:"library_ids"`
	MaxProfiles   types.Int64  `tfsdk:"max_profiles"`
	IsOwner       types.Bool   `tfsdk:"is_owner"`
	PasswordLogin types.Bool   `tfsdk:"password_login"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

var accountListItemType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id": types.StringType, "username": types.StringType, "email": types.StringType,
	"role": types.StringType, "enabled": types.BoolType,
	"permissions":     types.SetType{ElemType: types.StringType},
	"access_group_id": types.StringType, "library_ids": types.SetType{ElemType: types.StringType},
	"max_profiles": types.Int64Type, "is_owner": types.BoolType,
	"password_login": types.BoolType, "created_at": types.StringType, "updated_at": types.StringType,
}}

func (d *accountsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_accounts"
}
func (d *accountsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func() schema.StringAttribute { return schema.StringAttribute{Computed: true} }
	resp.Schema = schema.Schema{Description: "Lists login accounts on the Silo server.", Attributes: map[string]schema.Attribute{
		"accounts": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": str(), "username": str(), "email": str(), "role": str(),
			"enabled":         schema.BoolAttribute{Computed: true},
			"permissions":     schema.SetAttribute{Computed: true, ElementType: types.StringType},
			"access_group_id": str(),
			"library_ids":     schema.SetAttribute{Computed: true, ElementType: types.StringType},
			"max_profiles":    schema.Int64Attribute{Computed: true},
			"is_owner":        schema.BoolAttribute{Computed: true},
			"password_login":  schema.BoolAttribute{Computed: true},
			"created_at":      str(), "updated_at": str(),
		}}},
	}}
}
func (d *accountsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cl, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = cl
}
func (d *accountsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	cap, err := d.client.GetAccountCapabilities(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API error checking account capability", err.Error())
		return
	}
	if cap.State != "available" || !cap.Allowed {
		resp.Diagnostics.AddError("Account capability unavailable", fmt.Sprintf("state=%s allowed=%t", cap.State, cap.Allowed))
		return
	}
	accounts, err := d.client.ListAllAccounts(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API error listing accounts", err.Error())
		return
	}
	items := make([]accountListItemModel, 0, len(accounts))
	for _, a := range accounts {
		group := types.StringNull()
		if a.AccessGroupID != nil {
			group = types.StringValue(*a.AccessGroupID)
		}
		libraries := types.SetNull(types.StringType)
		if a.LibraryIDs != nil {
			libraries = stringSetFromSlice(ctx, *a.LibraryIDs)
		}
		items = append(items, accountListItemModel{
			ID: types.StringValue(a.ID), Username: types.StringValue(a.Username), Email: types.StringValue(a.Email),
			Role: types.StringValue(a.Role), Enabled: types.BoolValue(a.Enabled),
			Permissions: stringSetFromSlice(ctx, a.Permissions), AccessGroupID: group, LibraryIDs: libraries,
			MaxProfiles: types.Int64Value(a.MaxProfiles), IsOwner: types.BoolValue(a.IsOwner),
			PasswordLogin: types.BoolValue(a.PasswordLogin), CreatedAt: types.StringValue(a.CreatedAt), UpdatedAt: types.StringValue(a.UpdatedAt),
		})
	}
	list, diags := types.ListValueFrom(ctx, accountListItemType, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &accountsDataSourceModel{Accounts: list})...)
}
