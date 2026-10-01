package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/barolinecrewer/silo-server-tofu/internal/client"
)

// Ensure the resource satisfies the framework interfaces at compile time.
var (
	_ resource.Resource                = (*accessGroupResource)(nil)
	_ resource.ResourceWithConfigure   = (*accessGroupResource)(nil)
	_ resource.ResourceWithImportState = (*accessGroupResource)(nil)
)

// NewAccessGroupResource is the factory registered in provider.go.
func NewAccessGroupResource() resource.Resource {
	return &accessGroupResource{}
}

// accessGroupResource manages a Silo admin access group
// (POST/GET/PUT/DELETE /admin/access-groups). It is the pattern reference
// for every future resource: ETag-guarded writes, string IDs, and
// optional+computed attributes for server-defaulted fields.
type accessGroupResource struct {
	client *client.Client
}

// accessGroupResourceModel maps the resource state. Fields the API defaults
// are Optional+Computed; server-only fields (id, created_at, updated_at,
// etag) are Computed only.
type accessGroupResourceModel struct {
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
	CreatedAt                types.String `tfsdk:"created_at"`
	UpdatedAt                types.String `tfsdk:"updated_at"`
	ETag                     types.String `tfsdk:"etag"`
}

// Metadata sets the resource type name.
func (r *accessGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_group"
}

// Schema defines the resource schema. Every field the API returns is
// Optional+Computed (the server fills defaults on create); unknown plan
// values are omitted from create bodies, per AGENTS.md.
func (r *accessGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Silo admin access group: the policy bundle that limits " +
			"which libraries member accounts can see and how they may stream, " +
			"transcode, download, and submit requests.",
		MarkdownDescription: "Manages a Silo admin access group: the policy bundle that limits " +
			"which libraries member accounts can see and how they may stream, " +
			"transcode, download, and submit requests.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Opaque Silo access group identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the group.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Free-form description of the group.",
			},
			"library_ids": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "IDs of libraries the group can see. Null means the server default.",
			},
			"max_playback_quality": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Maximum playback quality for members of the group.",
			},
			"download_allowed": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether members may download.",
			},
			"download_transcode_allowed": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether members may download transcoded copies.",
			},
			"transcode_allowed": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether members may trigger transcodes.",
			},
			"audio_transcode_allowed": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether members may trigger audio transcodes.",
			},
			"max_streams": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Concurrent stream ceiling for the group.",
			},
			"max_transcodes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Concurrent transcode ceiling for the group.",
			},
			"max_remote_stream_bitrate_kbps": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Remote per-stream bitrate ceiling in kbps; 0 means unlimited.",
			},
			"max_local_stream_bitrate_kbps": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Local per-stream bitrate ceiling in kbps; 0 means unlimited.",
			},
			"allowed_permissions": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Assignable permissions granted to member accounts. Null means the server default.",
			},
			"requests_allowed": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether members may submit content requests.",
			},
			"is_default": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether new accounts land in this group by default.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC 3339 creation timestamp.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC 3339 last-update timestamp.",
			},
			"etag": schema.StringAttribute{
				Computed: true,
				Description: "Server ETag captured on the last read; sent back as " +
					"If-Match on guarded writes. Empty after an import without a refresh.",
			},
		},
	}
}

// Configure receives the shared API client from the provider.
func (r *accessGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.client = cl
}

// Create posts the group and stores the server's choices (defaults included)
// plus the response ETag in state.
func (r *accessGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accessGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	write, diags := accessGroupWriteFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, etag, err := r.client.CreateAccessGroup(ctx, write)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating access group",
			"Could not create access group "+plan.Name.ValueString()+": "+err.Error(),
		)
		return
	}

	accessGroupModelFromClient(ctx, group, etag, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes state. A 404 means the group is gone: remove it from state.
func (r *accessGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accessGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, etag, err := r.client.GetAccessGroup(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading access group",
			"Could not read access group "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	accessGroupModelFromClient(ctx, group, etag, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update full-replaces the group server-side, guarded by the stored ETag.
// A 412/428 means the group changed outside Terraform: fail loudly, never
// silently overwrite.
func (r *accessGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan accessGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state accessGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	write, diags := accessGroupWriteFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, etag, err := r.client.UpdateAccessGroup(ctx, state.ID.ValueString(), state.ETag.ValueString(), write)
	if err != nil {
		if client.IsPreconditionFailed(err) || client.IsPreconditionRequired(err) {
			resp.Diagnostics.AddError(
				"Access group changed outside Terraform",
				fmt.Sprintf(
					"The access group %s (%s) was modified outside Terraform, so this update was "+
						"rejected (If-Match precondition failed). Run `tofu refresh` to accept the "+
						"external changes into state, then apply again, or import the current resource.",
					plan.Name.ValueString(), state.ID.ValueString(),
				),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error updating access group",
			"Could not update access group "+plan.Name.ValueString()+": "+err.Error(),
		)
		return
	}

	accessGroupModelFromClient(ctx, group, etag, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the group, guarded by the stored ETag. A 404 is success.
func (r *accessGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accessGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteAccessGroup(ctx, state.ID.ValueString(), state.ETag.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		if client.IsPreconditionFailed(err) || client.IsPreconditionRequired(err) {
			resp.Diagnostics.AddError(
				"Access group changed outside Terraform",
				fmt.Sprintf(
					"The access group %s (%s) was modified outside Terraform, so this delete was "+
						"rejected (If-Match precondition failed). Run `tofu refresh`, then destroy again.",
					state.Name.ValueString(), state.ID.ValueString(),
				),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting access group",
			"Could not delete access group "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}
}

// ImportState brings an existing group into management by ID. The ETag is
// picked up by the next refresh.
func (r *accessGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// accessGroupWriteFromModel builds the API write body. Unknown or null
// plan values are omitted (server defaults apply on create); known values
// are sent explicitly. Per AGENTS.md, update bodies carry the full desired
// state because computed values come from prior state.
func accessGroupWriteFromModel(ctx context.Context, model *accessGroupResourceModel) (*client.AccessGroupWrite, diag.Diagnostics) {
	var diags diag.Diagnostics

	write := &client.AccessGroupWrite{
		Name: model.Name.ValueString(),
	}
	if !model.Description.IsUnknown() && !model.Description.IsNull() {
		write.Description = stringPtr(model.Description.ValueString())
	}
	if !model.LibraryIDs.IsUnknown() && !model.LibraryIDs.IsNull() {
		ids, d := stringSliceFromSet(ctx, model.LibraryIDs)
		diags.Append(d...)
		write.LibraryIDs = ids
	}
	if !model.MaxPlaybackQuality.IsUnknown() && !model.MaxPlaybackQuality.IsNull() {
		write.MaxPlaybackQuality = stringPtr(model.MaxPlaybackQuality.ValueString())
	}
	write.DownloadAllowed = boolPtrFromValue(model.DownloadAllowed)
	write.DownloadTranscodeAllowed = boolPtrFromValue(model.DownloadTranscodeAllowed)
	write.TranscodeAllowed = boolPtrFromValue(model.TranscodeAllowed)
	write.AudioTranscodeAllowed = boolPtrFromValue(model.AudioTranscodeAllowed)
	write.MaxStreams = int64PtrFromValue(model.MaxStreams)
	write.MaxTranscodes = int64PtrFromValue(model.MaxTranscodes)
	write.MaxRemoteBitrateKbps = int64PtrFromValue(model.MaxRemoteBitrateKbps)
	write.MaxLocalBitrateKbps = int64PtrFromValue(model.MaxLocalBitrateKbps)
	if !model.AllowedPermissions.IsUnknown() && !model.AllowedPermissions.IsNull() {
		perms, d := stringSliceFromSet(ctx, model.AllowedPermissions)
		diags.Append(d...)
		write.AllowedPermissions = perms
	}
	write.RequestsAllowed = boolPtrFromValue(model.RequestsAllowed)
	write.IsDefault = boolPtrFromValue(model.IsDefault)

	return write, diags
}

// accessGroupModelFromClient copies an API response (plus the read-time
// ETag) into the resource model, converting to tfsdk values.
func accessGroupModelFromClient(ctx context.Context, group *client.AccessGroup, etag string, model *accessGroupResourceModel) {
	model.ID = types.StringValue(group.ID)
	model.Name = types.StringValue(group.Name)
	model.Description = types.StringValue(group.Description)
	model.LibraryIDs = stringSetFromSlice(ctx, group.LibraryIDs)
	model.MaxPlaybackQuality = types.StringValue(group.MaxPlaybackQuality)
	model.DownloadAllowed = types.BoolValue(group.DownloadAllowed)
	model.DownloadTranscodeAllowed = types.BoolValue(group.DownloadTranscodeAllowed)
	model.TranscodeAllowed = types.BoolValue(group.TranscodeAllowed)
	model.AudioTranscodeAllowed = types.BoolValue(group.AudioTranscodeAllowed)
	model.MaxStreams = types.Int64Value(group.MaxStreams)
	model.MaxTranscodes = types.Int64Value(group.MaxTranscodes)
	model.MaxRemoteBitrateKbps = types.Int64Value(group.MaxRemoteBitrateKbps)
	model.MaxLocalBitrateKbps = types.Int64Value(group.MaxLocalBitrateKbps)
	model.AllowedPermissions = stringSetFromSlice(ctx, group.AllowedPermissions)
	model.RequestsAllowed = types.BoolValue(group.RequestsAllowed)
	model.IsDefault = types.BoolValue(group.IsDefault)
	model.CreatedAt = types.StringValue(group.CreatedAt)
	model.UpdatedAt = types.StringValue(group.UpdatedAt)
	if etag != "" {
		model.ETag = types.StringValue(etag)
	} else {
		model.ETag = types.StringNull()
	}
}
