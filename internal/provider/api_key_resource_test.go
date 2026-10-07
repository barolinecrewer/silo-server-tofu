package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// --- Offline: create-body construction -------------------------------------

func TestAPIKeyCreateFromModelOmitsUnknownAndNull(t *testing.T) {
	ctx := context.Background()
	model := &apiKeyResourceModel{
		Label:     types.StringValue("ci"),
		UserID:    types.StringUnknown(),
		Scopes:    types.SetUnknown(types.StringType),
		RateTier:  types.StringUnknown(),
		Key:       types.StringUnknown(),
		KeyPrefix: types.StringUnknown(),
		CreatedAt: types.StringUnknown(),
		ETag:      types.StringUnknown(),
	}

	write, diags := apiKeyCreateFromModel(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	buf, err := json.Marshal(write)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf) != `{"label":"ci"}` {
		t.Fatalf("expected omitted unknowns, got: %s", buf)
	}
}

func TestAPIKeyCreateFromModelSendsKnownValues(t *testing.T) {
	ctx := context.Background()
	scopes, diags := types.SetValueFrom(ctx, types.StringType, []string{"admin:read"})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	model := &apiKeyResourceModel{
		Label:  types.StringValue("ci"),
		UserID: types.StringValue("7"),
		Scopes: scopes,
	}

	write, diags := apiKeyCreateFromModel(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if write.UserID == nil || *write.UserID != "7" {
		t.Fatal("expected user_id to be sent")
	}
	if write.Scopes == nil || len(*write.Scopes) != 1 || (*write.Scopes)[0] != "admin:read" {
		t.Fatalf("expected scopes [admin:read], got %v", write.Scopes)
	}
}

// --- Acceptance: gated behind TF_ACC + env vars ------------------------------
//
// These tests create and destroy real API keys on a real server. Never point
// them at a production instance; they are destructive.

func TestAccAPIKeyResource(t *testing.T) {
	// Random labels keep a dangling key from an aborted run from colliding.
	label := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_7_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWrapper(`
resource "silo_api_key" "test" {
  label = "` + label + `"
}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("silo_api_key.test", "id", idPattern),
					resource.TestCheckResourceAttr("silo_api_key.test", "label", label),
					// The credential is disclosed once at creation and lands
					// in sensitive state.
					resource.TestMatchResourceAttr("silo_api_key.test", "key", idPattern),
				),
			},
			{
				// Update exercises the If-Match flow on the tier endpoint,
				// the only in-place updatable field.
				Config: testAccProviderConfigWrapper(`
resource "silo_api_key" "test" {
  label     = "` + label + `"
  rate_tier = "elevated"
}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("silo_api_key.test", "rate_tier", "elevated"),
					// The tier update did not replace the key, so the
					// credential is still in state.
					resource.TestMatchResourceAttr("silo_api_key.test", "key", idPattern),
				),
			},
			{
				// Import exercises the passthrough + read path. The etag is
				// refreshed and its exact value is not deterministic; the
				// credential is never disclosed again, so it cannot match.
				ResourceName:            "silo_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"etag", "key"},
			},
		},
	})
}

func TestAccAPIKeysDataSource(t *testing.T) {
	label := acctest.RandomWithPrefix("tf-acc-ds")
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_7_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWrapper(`
resource "silo_api_key" "test" {
  label = "` + label + `"
}

data "silo_api_keys" "all" {
  depends_on = [silo_api_key.test]
}

output "found" {
  value = contains([for k in data.silo_api_keys.all.api_keys : k.label], "` + label + `")
}`),
				Check: resource.TestCheckOutput("found", "true"),
			},
		},
	})
}
