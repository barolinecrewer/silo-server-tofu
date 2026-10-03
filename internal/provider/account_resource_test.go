package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccountBodyCreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	m := &accountModel{
		Username:      types.StringValue("alice"),
		Email:         types.StringValue("alice@example.test"),
		Role:          types.StringValue("user"),
		Enabled:       types.BoolValue(true),
		AccessGroupID: types.StringNull(),
		LibraryIDs:    types.SetNull(types.StringType),
		MaxStreams:    types.Int64Null(),
	}
	create, diags := accountBody(ctx, m, false)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if _, ok := create["enabled"]; ok {
		t.Fatal("create body must not send update-only enabled")
	}
	if _, ok := create["access_group_id"]; ok {
		t.Fatal("null override must be omitted on create")
	}
	update, diags := accountBody(ctx, m, true)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if update["enabled"] != true {
		t.Fatal("update must carry enabled")
	}
	if v, ok := update["access_group_id"]; !ok || v != nil {
		t.Fatal("update must carry explicit null access_group_id")
	}
	if v, ok := update["library_ids"]; !ok || v != nil {
		t.Fatal("update must carry explicit null library_ids")
	}
	if v, ok := update["max_streams"]; !ok || v != nil {
		t.Fatal("update must carry explicit null max_streams")
	}
}

// Runs only against a throwaway server configured with TF_ACC and SILO_ACC_*.
func TestAccAccountResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-user")
	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_7_0)},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []tfresource.TestStep{
			{
				Config: testAccProviderConfigWrapper(`
resource "silo_account" "test" {
  username               = "` + name + `"
  email                  = "` + name + `@example.test"
  password               = "temporary-secret-123"
  role                   = "user"
  create_default_profile = false
  max_streams            = 2
}`),
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestMatchResourceAttr("silo_account.test", "id", idPattern),
					tfresource.TestCheckResourceAttr("silo_account.test", "max_streams", "2"),
				),
			},
			{
				Config: testAccProviderConfigWrapper(`
resource "silo_account" "test" {
  username               = "` + name + `"
  email                  = "` + name + `@example.test"
  password               = "temporary-secret-123"
  role                   = "user"
  create_default_profile = false
  max_streams            = 3
}`),
				Check: tfresource.TestCheckResourceAttr("silo_account.test", "max_streams", "3"),
			},
			{
				Config: testAccProviderConfigWrapper(`
resource "silo_account" "test" {
  username               = "` + name + `"
  email                  = "` + name + `@example.test"
  password               = "rotated-secret-456"
  role                   = "user"
  create_default_profile = false
  max_streams            = 3
}`),
				Check: tfresource.TestCheckResourceAttr("silo_account.test", "password", "rotated-secret-456"),
			},
			{
				ResourceName:            "silo_account.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"etag", "password", "create_default_profile"},
			},
		},
	})
}

func TestAccAccountsDataSource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-list-user")
	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_7_0)},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []tfresource.TestStep{{
			Config: testAccProviderConfigWrapper(`
resource "silo_account" "test" {
  username               = "` + name + `"
  email                  = "` + name + `@example.test"
  password               = "temporary-secret-123"
  role                   = "user"
  create_default_profile = false
}

data "silo_accounts" "all" {
  depends_on = [silo_account.test]
}

output "found" {
  value = contains([for a in data.silo_accounts.all.accounts : a.username], "` + name + `")
}`),
			Check: tfresource.TestCheckOutput("found", "true"),
		}},
	})
}

func TestAccountResourceSchema(t *testing.T) {
	r := NewAccountResource()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !resp.Schema.Attributes["password"].IsSensitive() {
		t.Fatal("password must be sensitive")
	}
	if !resp.Schema.Attributes["etag"].IsComputed() {
		t.Fatal("etag must be computed")
	}
}
