package provider

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// --- Offline: write-body construction -------------------------------------

func TestAccessGroupWriteFromModelOmitsUnknownAndNull(t *testing.T) {
	ctx := context.Background()
	model := &accessGroupResourceModel{
		Name: types.StringValue("Family"),
		// Everything else unknown/null: omitted from the create body.
		Description:              types.StringUnknown(),
		LibraryIDs:               types.SetUnknown(types.StringType),
		MaxPlaybackQuality:       types.StringNull(),
		DownloadAllowed:          types.BoolUnknown(),
		DownloadTranscodeAllowed: types.BoolNull(),
		TranscodeAllowed:         types.BoolUnknown(),
		AudioTranscodeAllowed:    types.BoolNull(),
		MaxStreams:               types.Int64Unknown(),
		MaxTranscodes:            types.Int64Null(),
		MaxRemoteBitrateKbps:     types.Int64Unknown(),
		MaxLocalBitrateKbps:      types.Int64Null(),
		AllowedPermissions:       types.SetUnknown(types.StringType),
		RequestsAllowed:          types.BoolUnknown(),
		IsDefault:                types.BoolNull(),
	}

	write, diags := accessGroupWriteFromModel(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	buf, err := json.Marshal(write)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf) != `{"name":"Family"}` {
		t.Fatalf("expected omitted unknowns, got: %s", buf)
	}
}

func TestAccessGroupWriteFromModelSendsKnownValues(t *testing.T) {
	ctx := context.Background()
	libraryIDs, diags := types.SetValueFrom(ctx, types.StringType, []string{"1", "2"})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	model := &accessGroupResourceModel{
		Name:                 types.StringValue("Family"),
		Description:          types.StringValue("Household"),
		LibraryIDs:           libraryIDs,
		DownloadAllowed:      types.BoolValue(true),
		MaxStreams:           types.Int64Value(4),
		MaxRemoteBitrateKbps: types.Int64Value(0),
		RequestsAllowed:      types.BoolValue(true),
	}

	write, diags := accessGroupWriteFromModel(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if write.Name != "Family" {
		t.Fatalf("unexpected name: %s", write.Name)
	}
	if write.Description == nil || *write.Description != "Household" {
		t.Fatal("expected description to be sent")
	}
	if write.LibraryIDs == nil || len(*write.LibraryIDs) != 2 {
		t.Fatalf("expected 2 library IDs, got %v", write.LibraryIDs)
	}
	if write.DownloadAllowed == nil || !*write.DownloadAllowed {
		t.Fatal("expected download_allowed true")
	}
	if write.MaxStreams == nil || *write.MaxStreams != 4 {
		t.Fatalf("expected max_streams 4, got %v", write.MaxStreams)
	}
	// Explicit zero is a known value and must be sent, not omitted.
	if write.MaxRemoteBitrateKbps == nil || *write.MaxRemoteBitrateKbps != 0 {
		t.Fatal("expected explicit zero bitrate to be sent")
	}
}

// --- Acceptance: gated behind TF_ACC + env vars ----------------------------
//
// These tests create and destroy real resources on a real server. Never
// point them at a production instance; they are destructive.

// idPattern matches the API's opaque numeric-looking IDs without parsing
// them as numbers; the provider treats them as opaque strings.
var idPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set; skipping acceptance test")
	}
	if os.Getenv("SILO_ACC_BASE_URL") == "" || os.Getenv("SILO_ACC_API_KEY") == "" {
		t.Skip("SILO_ACC_BASE_URL and SILO_ACC_API_KEY not set; " +
			"acceptance tests need a throwaway test server")
	}
}

func testAccProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"silo": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func TestAccAccessGroupResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_7_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWrapper(`
resource "silo_access_group" "test" {
  name              = "tf-acc-test"
  description       = "created by acceptance tests"
  download_allowed  = true
  requests_allowed  = true
  max_streams       = 4
}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("silo_access_group.test", "id", idPattern),
					resource.TestCheckResourceAttr("silo_access_group.test", "name", "tf-acc-test"),
					resource.TestCheckResourceAttr("silo_access_group.test", "max_streams", "4"),
					resource.TestCheckNoResourceAttr("silo_access_group.test", "etag"),
				),
			},
			{
				// Update exercises the If-Match flow: the stored etag must
				// accompany the PUT and the new etag must land in state.
				Config: testAccProviderConfigWrapper(`
resource "silo_access_group" "test" {
  name              = "tf-acc-test"
  description       = "updated by acceptance tests"
  download_allowed  = true
  requests_allowed  = true
  max_streams       = 2
}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("silo_access_group.test", "max_streams", "2"),
				),
			},
			{
				// Import exercises the passthrough + read path.
				ResourceName:      "silo_access_group.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The etag is refreshed after import and its exact value is
				// not deterministic across servers.
				ImportStateVerifyIgnore: []string{"etag"},
			},
		},
	})
}

func TestAccAccessGroupsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_7_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWrapper(`
resource "silo_access_group" "test" {
  name = "tf-acc-ds-test"
}

data "silo_access_groups" "all" {}

output "found" {
  value = contains([for g in data.silo_access_groups.all.access_groups : g.name], "tf-acc-ds-test")
}`),
				Check: resource.TestCheckOutput("found", "true"),
			},
		},
	})
}

func testAccProviderConfigWrapper(body string) string {
	cfg := os.Getenv("SILO_ACC_BASE_URL")
	if cfg == "" {
		cfg = "http://localhost:8090"
	}
	apiKey := os.Getenv("SILO_ACC_API_KEY")
	return "provider \"silo\" {\n  base_url = \"" + cfg + "\"\n  api_key  = \"" + apiKey + "\"\n}\n" + body
}
