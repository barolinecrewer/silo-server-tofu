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
	_ datasource.DataSource              = (*accessGroupsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*accessGroupsDataSource)(nil)
)

// NewAccessGroupsDataSource is the factory registered in provider.go.
func NewAccessGroupsDataSource() datasource.DataSource {
	return &accessGroupsDataSource{}
}

// accessGroupsDataSource reads every access group on the server, walking
// the API's cursor pagination. It is the pattern reference for list-type
// data sources.
type accessGroupsDataSource struct {
	client *client.Client
}

// accessGroupsDataSourceModel is the data source state: one attribute
// holding every group.
type accessGroupsDataSourceModel struct {
	AccessGroups types.List `tfsdk:"access_groups"`
}

// accessGroupListItemModel is one element of the access_groups list.
type accessGroupListItemModel struct {
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	Description              types.String `tfsdk:"description"`
	LibraryIDs               types.Set    `tfsdk:"library_ids"`
	MaxPlaybackQuality       types.String `tfsdk:"max_playback_quality"`
	DownloadAllowed          types.Bool   `tfsdk:"download_allowed"`
	DownloadTranscodeAllowed types.Bool   `tfsdk:"download_transcode_allowed"`
	TranscodeAllowed         types.Bool   `tfsdk:"transcode_allowed"`
	AudioTranscodeAllowed    types.Bool   `tfsdk:"audio_transcode_allowed"`
	MaxStreams               types.Int64  `tfsdk:"max_streams"`
	MaxTranscodes            types.Int64  `tfsdk:"max_transcodes"`
	MaxRemoteBitrateKbps     types.Int64  `tfsdk:"max_remote_stream_bitrate_kbps"`
	MaxLocalBitrateKbps      types.Int64  `tfsdk:"max_local_stream_bitrate_kbps"`
	AllowedPermissions       types.Set    `tfsdk:"allowed_permissions"`
	RequestsAllowed          types.Bool   `tfsdk:"requests_allowed"`
	IsDefault                types.Bool   `tfsdk:"is_default"`
	MemberCount              types.Int64  `tfsdk:"member_count"`
	CreatedAt                types.String `tfsdk:"created_at"`
	UpdatedAt                types.String `tfsdk:"updated_at"`
}

// accessGroupListItemObjectType mirrors accessGroupListItemModel's
// tfsdk tags; keep the two in sync when adding fields.
var accessGroupListItemObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":                             types.StringType,
		"name":                           types.StringType,
		"description":                    types.StringType,
		"library_ids":                    types.SetType{ElemType: types.StringType},
		"max_playback_quality":           types.StringType,
		"download_allowed":               types.BoolType,
		"download_transcode_allowed":     types.BoolType,
		"transcode_allowed":              types.BoolType,
		"audio_transcode_allowed":        types.BoolType,
		"max_streams":                    types.Int64Type,
		"max_transcodes":                 types.Int64Type,
		"max_remote_stream_bitrate_kbps": types.Int64Type,
		"max_local_stream_bitrate_kbps":  types.Int64Type,
		"allowed_permissions":            types.SetType{ElemType: types.StringType},
		"requests_allowed":               types.BoolType,
		"is_default":                     types.BoolType,
		"member_count":                   types.Int64Type,
		"created_at":                     types.StringType,
		"updated_at":                     types.StringType,
	},
}

// Metadata sets the data source type name.
func (d *accessGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_groups"
}

// Schema defines the data source schema.
func (d *accessGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "All access groups on the server, with their policies. " +
			"Use to reference groups by name from other resources, or to audit policy.",
		Attributes: map[string]schema.Attribute{
			"access_groups": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every access group on the server.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Opaque Silo access group identifier.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Display name of the group.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "Free-form description of the group.",
						},
						"library_ids": schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "IDs of libraries the group can see.",
						},
						"max_playback_quality": schema.StringAttribute{
							Computed:    true,
							Description: "Maximum playback quality for members.",
						},
						"download_allowed": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether members may download.",
						},
						"download_transcode_allowed": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether members may download transcoded copies.",
						},
						"transcode_allowed": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether members may trigger transcodes.",
						},
						"audio_transcode_allowed": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether members may trigger audio transcodes.",
						},
						"max_streams": schema.Int64Attribute{
							Computed:    true,
							Description: "Concurrent stream ceiling for the group.",
						},
						"max_transcodes": schema.Int64Attribute{
							Computed:    true,
							Description: "Concurrent transcode ceiling for the group.",
						},
						"max_remote_stream_bitrate_kbps": schema.Int64Attribute{
							Computed:    true,
							Description: "Remote per-stream bitrate ceiling in kbps; 0 means unlimited.",
						},
						"max_local_stream_bitrate_kbps": schema.Int64Attribute{
							Computed:    true,
							Description: "Local per-stream bitrate ceiling in kbps; 0 means unlimited.",
						},
						"allowed_permissions": schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Assignable permissions granted to member accounts.",
						},
						"requests_allowed": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether members may submit content requests.",
						},
						"is_default": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether new accounts land in this group by default.",
						},
						"member_count": schema.Int64Attribute{
							Computed:    true,
							Description: "Live member count at read time; do not depend on it for plans.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "RFC 3339 creation timestamp.",
						},
						"updated_at": schema.StringAttribute{
							Computed:    true,
							Description: "RFC 3339 last-update timestamp.",
						},
					},
				},
			},
		},
	}
}

// Configure receives the shared API client from the provider.
func (d *accessGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read walks every page of the access-group collection into state.
func (d *accessGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	groups, err := d.client.ListAllAccessGroups(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing access groups",
			"Could not list access groups: "+err.Error(),
		)
		return
	}

	elements := make([]accessGroupListItemModel, 0, len(groups))
	for i := range groups {
		elements = append(elements, accessGroupListItemModel{
			ID:                       types.StringValue(groups[i].ID),
			Name:                     types.StringValue(groups[i].Name),
			Description:              types.StringValue(groups[i].Description),
			LibraryIDs:               stringSetFromSlice(ctx, groups[i].LibraryIDs),
			MaxPlaybackQuality:       types.StringValue(groups[i].MaxPlaybackQuality),
			DownloadAllowed:          types.BoolValue(groups[i].DownloadAllowed),
			DownloadTranscodeAllowed: types.BoolValue(groups[i].DownloadTranscodeAllowed),
			TranscodeAllowed:         types.BoolValue(groups[i].TranscodeAllowed),
			AudioTranscodeAllowed:    types.BoolValue(groups[i].AudioTranscodeAllowed),
			MaxStreams:               types.Int64Value(groups[i].MaxStreams),
			MaxTranscodes:            types.Int64Value(groups[i].MaxTranscodes),
			MaxRemoteBitrateKbps:     types.Int64Value(groups[i].MaxRemoteBitrateKbps),
			MaxLocalBitrateKbps:      types.Int64Value(groups[i].MaxLocalBitrateKbps),
			AllowedPermissions:       stringSetFromSlice(ctx, groups[i].AllowedPermissions),
			RequestsAllowed:          types.BoolValue(groups[i].RequestsAllowed),
			IsDefault:                types.BoolValue(groups[i].IsDefault),
			MemberCount:              types.Int64Value(groups[i].MemberCount),
			CreatedAt:                types.StringValue(groups[i].CreatedAt),
			UpdatedAt:                types.StringValue(groups[i].UpdatedAt),
		})
	}

	list, diags := types.ListValueFrom(ctx, accessGroupListItemObjectType, elements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := accessGroupsDataSourceModel{AccessGroups: list}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
