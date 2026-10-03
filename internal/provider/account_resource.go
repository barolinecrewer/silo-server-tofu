package provider

import (
	"context"
	"fmt"

	"github.com/barolinecrewer/silo-server-tofu/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*accountResource)(nil)
var _ resource.ResourceWithConfigure = (*accountResource)(nil)
var _ resource.ResourceWithImportState = (*accountResource)(nil)

func NewAccountResource() resource.Resource { return &accountResource{} }

type accountResource struct{ client *client.Client }

type accountModel struct {
	ID                         types.String `tfsdk:"id"`
	Username                   types.String `tfsdk:"username"`
	Email                      types.String `tfsdk:"email"`
	Password                   types.String `tfsdk:"password"`
	Role                       types.String `tfsdk:"role"`
	Enabled                    types.Bool   `tfsdk:"enabled"`
	Permissions                types.Set    `tfsdk:"permissions"`
	AccessGroupID              types.String `tfsdk:"access_group_id"`
	LibraryIDs                 types.Set    `tfsdk:"library_ids"`
	MaxPlaybackQuality         types.String `tfsdk:"max_playback_quality"`
	MaxStreams                 types.Int64  `tfsdk:"max_streams"`
	MaxTranscodes              types.Int64  `tfsdk:"max_transcodes"`
	MaxRemoteStreamBitrateKbps types.Int64  `tfsdk:"max_remote_stream_bitrate_kbps"`
	MaxLocalStreamBitrateKbps  types.Int64  `tfsdk:"max_local_stream_bitrate_kbps"`
	TranscodeAllowed           types.Bool   `tfsdk:"transcode_allowed"`
	AudioTranscodeAllowed      types.Bool   `tfsdk:"audio_transcode_allowed"`
	DownloadAllowed            types.Bool   `tfsdk:"download_allowed"`
	DownloadTranscodeAllowed   types.Bool   `tfsdk:"download_transcode_allowed"`
	RequestsAllowed            types.Bool   `tfsdk:"requests_allowed"`
	MaxProfiles                types.Int64  `tfsdk:"max_profiles"`
	CreateDefaultProfile       types.Bool   `tfsdk:"create_default_profile"`
	DefaultProfileName         types.String `tfsdk:"default_profile_name"`
	RequirePasswordChange      types.Bool   `tfsdk:"require_password_change"`
	PasswordLogin              types.Bool   `tfsdk:"password_login"`
	PasswordChangeRequired     types.Bool   `tfsdk:"password_change_required"`
	IsOwner                    types.Bool   `tfsdk:"is_owner"`
	CreatedAt                  types.String `tfsdk:"created_at"`
	UpdatedAt                  types.String `tfsdk:"updated_at"`
	LastActiveAt               types.String `tfsdk:"last_active_at"`
	ETag                       types.String `tfsdk:"etag"`
}

func (r *accountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (r *accountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	str := func(d string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Description: d}
	}
	boolean := func(d string) schema.BoolAttribute {
		return schema.BoolAttribute{Optional: true, Computed: true, Description: d}
	}
	number := func(d string) schema.Int64Attribute {
		return schema.Int64Attribute{Optional: true, Computed: true, Description: d}
	}
	set := func(d string) schema.SetAttribute {
		return schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, Description: d}
	}
	computed := func(d string) schema.StringAttribute { return schema.StringAttribute{Computed: true, Description: d} }
	resp.Schema = schema.Schema{Description: "Manages a Silo login account.", Attributes: map[string]schema.Attribute{
		"id":                             schema.StringAttribute{Computed: true, Description: "Opaque account ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"username":                       schema.StringAttribute{Required: true, Description: "Login username."},
		"email":                          schema.StringAttribute{Required: true, Description: "Contact email."},
		"password":                       schema.StringAttribute{Optional: true, Sensitive: true, Description: "Password, required on create. Changing it rotates the password; the value is retained in sensitive state."},
		"role":                           schema.StringAttribute{Required: true, Description: "Account role: admin or user."},
		"enabled":                        boolean("Whether the account can sign in."),
		"permissions":                    set("Permissions assigned directly to the account."),
		"access_group_id":                str("Access group ID; null means no group."),
		"library_ids":                    set("Explicit library allowlist; null inherits from the group."),
		"max_playback_quality":           str("Playback ceiling override; null inherits."),
		"max_streams":                    number("Stream limit override; null inherits."),
		"max_transcodes":                 number("Transcode limit override; null inherits."),
		"max_remote_stream_bitrate_kbps": number("Remote stream bitrate override in kbps; null inherits."),
		"max_local_stream_bitrate_kbps":  number("Local stream bitrate override in kbps; null inherits."),
		"transcode_allowed":              boolean("Transcode permission override; null inherits."),
		"audio_transcode_allowed":        boolean("Audio transcode permission override; null inherits."),
		"download_allowed":               boolean("Download permission override; null inherits."),
		"download_transcode_allowed":     boolean("Transcoded download permission override; null inherits."),
		"requests_allowed":               boolean("Request permission override; null inherits."),
		"max_profiles":                   number("Maximum household profiles."),
		"create_default_profile":         schema.BoolAttribute{Optional: true, Description: "Create a default profile with the account; defaults to false.", PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()}},
		"default_profile_name":           schema.StringAttribute{Optional: true, Description: "Name of the default profile on creation.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"require_password_change":        schema.BoolAttribute{Optional: true, Description: "Require a password change after setting or rotating a password."},
		"password_login":                 schema.BoolAttribute{Computed: true, Description: "Whether local password sign-in is active."},
		"password_change_required":       schema.BoolAttribute{Computed: true, Description: "Whether the account must change its password."},
		"is_owner":                       schema.BoolAttribute{Computed: true, Description: "Whether this is the server owner account."},
		"created_at":                     computed("Creation timestamp."),
		"updated_at":                     computed("Last update timestamp."),
		"last_active_at":                 computed("Last activity timestamp, if any."),
		"etag":                           computed("Current ETag used for guarded writes."),
	}}
}

func (r *accountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cl, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = cl
}

func (r *accountResource) checkCapability(ctx context.Context) error {
	cap, err := r.client.GetAccountCapabilities(ctx)
	if err != nil {
		return err
	}
	if cap.State != "available" || !cap.Allowed {
		return fmt.Errorf("account capability is %s (allowed=%t)", cap.State, cap.Allowed)
	}
	return nil
}

func (r *accountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Password.IsNull() || plan.Password.IsUnknown() {
		resp.Diagnostics.AddError("Missing account password", "Set password when creating an account.")
		return
	}
	if plan.CreateDefaultProfile.IsUnknown() {
		resp.Diagnostics.AddError("Unknown default profile choice", "Set create_default_profile to true or false.")
		return
	}
	if err := r.checkCapability(ctx); err != nil {
		resp.Diagnostics.AddError("Account capability unavailable", err.Error())
		return
	}
	body, diags := accountBody(ctx, &plan, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body["password"] = plan.Password.ValueString()
	body["create_default_profile"] = !plan.CreateDefaultProfile.IsNull() && plan.CreateDefaultProfile.ValueBool()
	if !plan.DefaultProfileName.IsNull() && !plan.DefaultProfileName.IsUnknown() {
		body["default_profile_name"] = plan.DefaultProfileName.ValueString()
	}
	if !plan.RequirePasswordChange.IsNull() && !plan.RequirePasswordChange.IsUnknown() {
		body["require_password_change"] = plan.RequirePasswordChange.ValueBool()
	}
	id, err := r.client.CreateAccount(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("API error creating account", err.Error())
		return
	}
	plan.ID = types.StringValue(id)
	// Save the ID before reading, so a failed read leaves a recoverable resource.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	account, etag, err := r.client.GetAccount(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API error reading created account", err.Error())
		return
	}
	accountModelFromClient(ctx, account, etag, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	account, etag, err := r.client.GetAccount(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("API error reading account", err.Error())
		return
	}
	accountModelFromClient(ctx, account, etag, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state accountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.checkCapability(ctx); err != nil {
		resp.Diagnostics.AddError("Account capability unavailable", err.Error())
		return
	}
	accountFillUnknown(&plan, &state)
	body, diags := accountBody(ctx, &plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	passwordChanged := !plan.Password.IsNull() && !plan.Password.IsUnknown() && !plan.Password.Equal(state.Password)
	if passwordChanged {
		body["password"] = plan.Password.ValueString()
		if !plan.RequirePasswordChange.IsNull() && !plan.RequirePasswordChange.IsUnknown() {
			body["require_password_change"] = plan.RequirePasswordChange.ValueBool()
		}
	} else if !plan.RequirePasswordChange.Equal(state.RequirePasswordChange) {
		resp.Diagnostics.AddError("Password change required", "Change password together with require_password_change; the API accepts that flag only with a password update.")
		return
	}
	if err := r.client.UpdateAccount(ctx, state.ID.ValueString(), state.ETag.ValueString(), body); err != nil {
		accountWriteDiagnostic(&resp.Diagnostics, "updating", err)
		return
	}
	account, etag, err := r.client.GetAccount(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API error reading updated account", err.Error())
		return
	}
	accountModelFromClient(ctx, account, etag, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Optional computed values may be unknown in an update plan. The API update
// body represents the full desired configuration, so retain their last read
// values rather than converting unknowns or clearing server policy.
func accountFillUnknown(plan, state *accountModel) {
	if plan.Enabled.IsUnknown() {
		plan.Enabled = state.Enabled
	}
	if plan.Permissions.IsUnknown() {
		plan.Permissions = state.Permissions
	}
	if plan.AccessGroupID.IsUnknown() {
		plan.AccessGroupID = state.AccessGroupID
	}
	if plan.LibraryIDs.IsUnknown() {
		plan.LibraryIDs = state.LibraryIDs
	}
	if plan.MaxPlaybackQuality.IsUnknown() {
		plan.MaxPlaybackQuality = state.MaxPlaybackQuality
	}
	if plan.MaxStreams.IsUnknown() {
		plan.MaxStreams = state.MaxStreams
	}
	if plan.MaxTranscodes.IsUnknown() {
		plan.MaxTranscodes = state.MaxTranscodes
	}
	if plan.MaxRemoteStreamBitrateKbps.IsUnknown() {
		plan.MaxRemoteStreamBitrateKbps = state.MaxRemoteStreamBitrateKbps
	}
	if plan.MaxLocalStreamBitrateKbps.IsUnknown() {
		plan.MaxLocalStreamBitrateKbps = state.MaxLocalStreamBitrateKbps
	}
	if plan.TranscodeAllowed.IsUnknown() {
		plan.TranscodeAllowed = state.TranscodeAllowed
	}
	if plan.AudioTranscodeAllowed.IsUnknown() {
		plan.AudioTranscodeAllowed = state.AudioTranscodeAllowed
	}
	if plan.DownloadAllowed.IsUnknown() {
		plan.DownloadAllowed = state.DownloadAllowed
	}
	if plan.DownloadTranscodeAllowed.IsUnknown() {
		plan.DownloadTranscodeAllowed = state.DownloadTranscodeAllowed
	}
	if plan.RequestsAllowed.IsUnknown() {
		plan.RequestsAllowed = state.RequestsAllowed
	}
	if plan.MaxProfiles.IsUnknown() {
		plan.MaxProfiles = state.MaxProfiles
	}
}

func (r *accountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteAccount(ctx, state.ID.ValueString(), state.ETag.ValueString()); err != nil {
		accountWriteDiagnostic(&resp.Diagnostics, "deleting", err)
	}
}

func (r *accountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func accountWriteDiagnostic(diags *diag.Diagnostics, action string, err error) {
	if client.IsPreconditionFailed(err) || client.IsPreconditionRequired(err) {
		diags.AddError("Account changed outside OpenTofu", "The account's ETag is stale or missing. Run `tofu refresh` and retry, or import the current account. "+err.Error())
		return
	}
	diags.AddError("API error "+action+" account", err.Error())
}

func accountBody(ctx context.Context, m *accountModel, update bool) (map[string]any, diag.Diagnostics) {
	body := map[string]any{"username": m.Username.ValueString(), "email": m.Email.ValueString(), "role": m.Role.ValueString()}
	var diags diag.Diagnostics
	putString := func(key string, v types.String) {
		if update || (!v.IsNull() && !v.IsUnknown()) {
			if v.IsNull() {
				body[key] = nil
			} else {
				body[key] = v.ValueString()
			}
		}
	}
	putBool := func(key string, v types.Bool) {
		if update || (!v.IsNull() && !v.IsUnknown()) {
			if v.IsNull() {
				body[key] = nil
			} else {
				body[key] = v.ValueBool()
			}
		}
	}
	putInt := func(key string, v types.Int64) {
		if update || (!v.IsNull() && !v.IsUnknown()) {
			if v.IsNull() {
				body[key] = nil
			} else {
				body[key] = v.ValueInt64()
			}
		}
	}
	putSet := func(key string, v types.Set) {
		if update || (!v.IsNull() && !v.IsUnknown()) {
			if v.IsNull() {
				body[key] = nil
			} else {
				list, d := stringSliceFromSet(ctx, v)
				diags.Append(d...)
				if list != nil {
					if *list == nil {
						body[key] = []string{}
					} else {
						body[key] = *list
					}
				}
			}
		}
	}
	if update {
		putBool("enabled", m.Enabled)
	}
	putSet("permissions", m.Permissions)
	putString("access_group_id", m.AccessGroupID)
	putSet("library_ids", m.LibraryIDs)
	putString("max_playback_quality", m.MaxPlaybackQuality)
	putInt("max_streams", m.MaxStreams)
	putInt("max_transcodes", m.MaxTranscodes)
	putInt("max_remote_stream_bitrate_kbps", m.MaxRemoteStreamBitrateKbps)
	putInt("max_local_stream_bitrate_kbps", m.MaxLocalStreamBitrateKbps)
	putBool("transcode_allowed", m.TranscodeAllowed)
	putBool("audio_transcode_allowed", m.AudioTranscodeAllowed)
	putBool("download_allowed", m.DownloadAllowed)
	putBool("download_transcode_allowed", m.DownloadTranscodeAllowed)
	putBool("requests_allowed", m.RequestsAllowed)
	putInt("max_profiles", m.MaxProfiles)
	if update {
		delete(body, "password")
		if m.Enabled.IsNull() {
			delete(body, "enabled")
		}
		if m.MaxProfiles.IsNull() {
			delete(body, "max_profiles")
		}
		if m.Permissions.IsNull() {
			delete(body, "permissions")
		}
	}
	return body, diags
}

func accountModelFromClient(ctx context.Context, a *client.Account, etag string, m *accountModel) {
	m.ID = types.StringValue(a.ID)
	m.Username = types.StringValue(a.Username)
	m.Email = types.StringValue(a.Email)
	m.Role = types.StringValue(a.Role)
	m.Enabled = types.BoolValue(a.Enabled)
	m.Permissions = stringSetFromSlice(ctx, a.Permissions)
	str := func(p *string) types.String {
		if p == nil {
			return types.StringNull()
		}
		return types.StringValue(*p)
	}
	boolean := func(p *bool) types.Bool {
		if p == nil {
			return types.BoolNull()
		}
		return types.BoolValue(*p)
	}
	number := func(p *int64) types.Int64 {
		if p == nil {
			return types.Int64Null()
		}
		return types.Int64Value(*p)
	}
	m.AccessGroupID = str(a.AccessGroupID)
	if a.LibraryIDs == nil {
		m.LibraryIDs = types.SetNull(types.StringType)
	} else {
		m.LibraryIDs = stringSetFromSlice(ctx, *a.LibraryIDs)
	}
	m.MaxPlaybackQuality = str(a.MaxPlaybackQuality)
	m.MaxStreams = number(a.MaxStreams)
	m.MaxTranscodes = number(a.MaxTranscodes)
	m.MaxRemoteStreamBitrateKbps = number(a.MaxRemoteStreamBitrateKbps)
	m.MaxLocalStreamBitrateKbps = number(a.MaxLocalStreamBitrateKbps)
	m.TranscodeAllowed = boolean(a.TranscodeAllowed)
	m.AudioTranscodeAllowed = boolean(a.AudioTranscodeAllowed)
	m.DownloadAllowed = boolean(a.DownloadAllowed)
	m.DownloadTranscodeAllowed = boolean(a.DownloadTranscodeAllowed)
	m.RequestsAllowed = boolean(a.RequestsAllowed)
	m.MaxProfiles = types.Int64Value(a.MaxProfiles)
	m.PasswordLogin = types.BoolValue(a.PasswordLogin)
	m.PasswordChangeRequired = types.BoolValue(a.PasswordChangeRequired)
	m.IsOwner = types.BoolValue(a.IsOwner)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	m.LastActiveAt = str(a.LastActiveAt)
	m.ETag = types.StringValue(etag)
}
