package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

// Offline tests: no server, no network. They pin the provider metadata and
// schema basics every future change must preserve.

func TestProviderMetadata(t *testing.T) {
	p := &siloProvider{}
	req := provider.MetadataRequest{}
	resp := &provider.MetadataResponse{}
	p.Metadata(context.Background(), req, resp)
	if resp.TypeName != providerTypeName {
		t.Fatalf("expected provider type name %q, got %q", providerTypeName, resp.TypeName)
	}
}

func TestProviderSchema(t *testing.T) {
	p := &siloProvider{}
	req := provider.SchemaRequest{}
	resp := &provider.SchemaResponse{}
	p.Schema(context.Background(), req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema returned diagnostics: %+v", resp.Diagnostics)
	}
	for _, name := range []string{"base_url", "api_key"} {
		attr, ok := resp.Schema.Attributes[name]
		if !ok {
			t.Fatalf("expected schema attribute %q", name)
		}
		if !attr.IsRequired() {
			t.Fatalf("expected attribute %q to be required", name)
		}
	}
	if !resp.Schema.Attributes["api_key"].IsSensitive() {
		t.Fatal("api_key must be sensitive")
	}
}

func TestProviderRegistries(t *testing.T) {
	p := &siloProvider{}
	resources := p.Resources(context.Background())
	if len(resources) == 0 {
		t.Fatal("expected at least one registered resource")
	}
	dataSources := p.DataSources(context.Background())
	if len(dataSources) == 0 {
		t.Fatal("expected at least one registered data source")
	}
}
