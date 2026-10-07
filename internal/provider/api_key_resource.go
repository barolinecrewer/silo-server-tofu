package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/barolinecrewer/silo-server-tofu/internal/client"
)

// Ensure the resource satisfies the framework interfaces at compile time.
var (
	_ resource.Resource                = (*apiKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*apiKeyResource)(nil)
	_ resource.ResourceWithImportState = (*apiKeyResource)(nil)
)

// NewAPIKeyResource is the factory registered in provider.go.
func NewAPIKeyResource() resource.Resource {
	return &apiKeyResource{}
}

// apiKeyResource manages a Silo admin API key
// (POST/GET/DELETE /admin/api-keys, PUT /admin/api-keys/{id}/tier). Only the
// rate tier is updatable in place; label, scopes, and the owning account are
// set at create only, so changing them replaces the key. The credential is
// disclosed once at creation and is kept in sensitive state.
type apiKeyResource struct {
	client *client.Client
}

// apiKeyResourceModel maps the resource state. Create-only fields are
// Optional+Computed (the server defaults apply when omitted); the one-time
// key and the read-only prefix are Computed.
type apiKeyResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Label     types.String `tfsdk:"label"`
	UserID    types.String `tfsdk:"user_id"`
	Scopes    types.Set    `tfsdk:"scopes"`
	RateTier  types.String `tfsdk:"rate_tier"`
	Key       types.String `tfsdk:"key"`
	KeyPrefix types.String `tfsdk:"key_prefix"`
	CreatedAt types.String `tfsdk:"created_at"`
	ETag      types.String `tfsdk:"etag"`
}

// Metadata sets the resource type name.
func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

// Schema defines the resource schema. The key is sensitive: it is stored in
// state so it can be used downstream, but never shown in plans or logs.
func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	description := "Manages a Silo admin API key. The credential value is disclosed " +
		"only once, at creation, and is stored in sensitive state. Label, scopes, " +
		"and the owning account cannot be changed after creation; changing them " +
		"replaces the key (and its credential)."
	resp.Schema = schema.Schema{
		Description:         description,
		MarkdownDescription: description,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Opaque Silo API key identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"label": schema.StringAttribute{
				Required:    true,
				Description: "Display label for the key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Opaque ID of the account owning the key. Null means the authenticated account; keys inherit their owner's permissions.",
				PlanModifiers: []planmodifier.String{
					// Create-only field. RequiresReplaceIfConfigured rather
					// than RequiresReplace: Optional+Computed attributes are
					// marked unknown during planning when unconfigured, and
					// the stock modifier would read that as a change and
					// replace the key on every apply.
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"scopes": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Scopes granted to the key. Null means the server default.",
				PlanModifiers: []planmodifier.Set{
					// See user_id: create-only, unconfigured means keep the
					// server default rather than replace.
					setplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"rate_tier": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Rate limit tier of the key: \"standard\" or \"elevated\". This is the only updatable field.",
				Validators: []validator.String{
					stringvalidator.OneOf("standard", "elevated"),
				},
			},
			"key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The credential value, disclosed only in the creation response and stored in sensitive state. Unknown after import.",
			},
			"key_prefix": schema.StringAttribute{
				Computed:    true,
				Description: "Non-secret prefix of the credential, as shown on later reads.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC 3339 creation timestamp.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// checkCapability reads the domain capability document: a disabled or
// unsupported feature is a configuration problem, not a failed request.
func (r *apiKeyResource) checkCapability(ctx context.Context) error {
	cap, err := r.client.GetAPIKeyCapabilities(ctx)
	if err != nil {
		return err
	}
	if cap.State != "available" || !cap.Allowed {
		return fmt.Errorf("api key capability is %s (allowed=%t)", cap.State, cap.Allowed)
	}
	return nil
}

// Create posts the key, keeps the one-time credential in state, then reads
// the key back for its ETag (the 201 carries only Location).
func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.checkCapability(ctx); err != nil {
		resp.Diagnostics.AddError("API key capability unavailable", err.Error())
		return
	}

	write, diags := apiKeyCreateFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateAPIKey(ctx, write)
	if err != nil {
		resp.Diagnostics.AddError(
			"API error creating api key",
			"Could not create api key "+plan.Label.ValueString()+": "+err.Error(),
		)
		return
	}

	apiKeyModelFromCreated(ctx, created, &plan)
	// Save state before reading: a failed read taints the resource instead of
	// orphaning a live credential.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, etag, err := r.client.GetAPIKey(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"API error reading created api key",
			"Created api key "+plan.ID.ValueString()+" but could not read its ETag: "+err.Error(),
		)
		return
	}
	apiKeyModelFromClient(ctx, key, etag, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes state. A 404 means the key is gone: remove it from state.
// The credential is never returned after creation, so the state value (if
// any) is preserved.
func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, etag, err := r.client.GetAPIKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"API error reading api key",
			"Could not read api key "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	apiKeyModelFromClient(ctx, key, etag, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update sets the rate tier via the guarded tier endpoint. All other
// attributes are create-only and force a replacement instead.
func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// rate_tier is Optional+Computed: an unconfigured plan value falls back
	// to the tier already in state.
	rateTier := plan.RateTier
	if rateTier.IsUnknown() || rateTier.IsNull() {
		rateTier = state.RateTier
	}

	if err := r.checkCapability(ctx); err != nil {
		resp.Diagnostics.AddError("API key capability unavailable", err.Error())
		return
	}

	key, etag, err := r.client.UpdateAPIKeyTier(ctx, state.ID.ValueString(), state.ETag.ValueString(), rateTier.ValueString())
	if err != nil {
		if client.IsPreconditionFailed(err) || client.IsPreconditionRequired(err) {
			resp.Diagnostics.AddError(
				"API key changed outside Terraform",
				fmt.Sprintf(
					"The API key %s (%s) was modified outside Terraform, so this update was "+
						"rejected (If-Match precondition failed). Run `tofu refresh` to accept the "+
						"external changes into state, then apply again, or import the current resource.",
					plan.Label.ValueString(), state.ID.ValueString(),
				),
			)
			return
		}
		resp.Diagnostics.AddError(
			"API error updating api key",
			"Could not update api key "+plan.Label.ValueString()+": "+err.Error(),
		)
		return
	}

	// The tier response is the read shape; the one-time credential is not
	// part of it. It is Computed with null config, so the update plan carries
	// it as unknown: restore the value from prior state, which Update ran
	// against (an in-place tier change never rotates the credential).
	if plan.Key.IsUnknown() || plan.Key.IsNull() {
		plan.Key = state.Key
	}
	apiKeyModelFromClient(ctx, key, etag, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the key, guarded by the stored ETag. A 404 is success.
func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteAPIKey(ctx, state.ID.ValueString(), state.ETag.ValueString()); err != nil {
		if client.IsPreconditionFailed(err) || client.IsPreconditionRequired(err) {
			resp.Diagnostics.AddError(
				"API key changed outside Terraform",
				fmt.Sprintf(
					"The API key %s (%s) was modified outside Terraform, so this delete was "+
						"rejected (If-Match precondition failed). Run `tofu refresh`, then destroy again.",
					state.Label.ValueString(), state.ID.ValueString(),
				),
			)
			return
		}
		resp.Diagnostics.AddError(
			"API error deleting api key",
			"Could not delete api key "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}
}

// ImportState brings an existing key into management by ID. The ETag is
// picked up by the next refresh; the one-time credential stays unknown.
func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// apiKeyCreateFromModel builds the create body. Unknown or null plan values
// are omitted (server defaults apply).
func apiKeyCreateFromModel(ctx context.Context, model *apiKeyResourceModel) (*client.APIKeyCreate, diag.Diagnostics) {
	var diags diag.Diagnostics

	write := &client.APIKeyCreate{
		Label: model.Label.ValueString(),
	}
	if !model.UserID.IsUnknown() && !model.UserID.IsNull() {
		write.UserID = stringPtr(model.UserID.ValueString())
	}
	if !model.Scopes.IsUnknown() && !model.Scopes.IsNull() {
		scopes, d := stringSliceFromSet(ctx, model.Scopes)
		diags.Append(d...)
		write.Scopes = scopes
	}

	return write, diags
}

// apiKeyModelFromCreated copies the one-time creation payload into the
// resource model.
func apiKeyModelFromCreated(ctx context.Context, created *client.APIKeyCreated, model *apiKeyResourceModel) {
	model.ID = types.StringValue(created.ID)
	model.UserID = types.StringValue(created.UserID)
	model.Label = types.StringValue(created.Label)
	model.Key = types.StringValue(created.Key)
	model.RateTier = types.StringValue(created.RateTier)
	model.Scopes = stringSetFromSlice(ctx, created.Scopes)
	model.CreatedAt = types.StringValue(created.CreatedAt)
}

// apiKeyModelFromClient copies an API response (plus the read-time ETag) into
// the resource model. The credential is never touched: reads do not return
// it, so the state value survives refreshes.
func apiKeyModelFromClient(ctx context.Context, key *client.APIKey, etag string, model *apiKeyResourceModel) {
	model.ID = types.StringValue(key.ID)
	model.UserID = types.StringValue(key.UserID)
	model.Label = types.StringValue(key.Label)
	model.KeyPrefix = types.StringValue(key.KeyPrefix)
	model.RateTier = types.StringValue(key.RateTier)
	model.Scopes = stringSetFromSlice(ctx, key.Scopes)
	model.CreatedAt = types.StringValue(key.CreatedAt)
	if etag != "" {
		model.ETag = types.StringValue(etag)
	} else {
		model.ETag = types.StringNull()
	}
}
