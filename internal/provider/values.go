package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Shared conversions between tfsdk values and plain Go values used in API
// bodies and responses. These encode the AGENTS.md conventions: unknown or
// null becomes an omitted field (nil pointer), and lists become tfsdk sets
// with a null for the API's null/absent case.

func stringPtr(v string) *string {
	return &v
}

// boolPtrFromValue maps a tfsdk Bool to a write-body pointer: unknown or
// null is omitted; known values are sent explicitly.
func boolPtrFromValue(v types.Bool) *bool {
	if v.IsUnknown() || v.IsNull() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// int64PtrFromValue maps a tfsdk Int64 to a write-body pointer: unknown or
// null is omitted.
func int64PtrFromValue(v types.Int64) *int64 {
	if v.IsUnknown() || v.IsNull() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

// stringSliceFromSet converts a tfsdk set of strings to a *[]string write
// field. The caller has already checked for unknown/null.
func stringSliceFromSet(ctx context.Context, s types.Set) (*[]string, diag.Diagnostics) {
	var out []string
	diags := s.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return nil, diags
	}
	return &out, nil
}

// stringSetFromSlice converts an API list to a tfsdk set of strings. A nil
// slice (the API's null/absent case) becomes a null set.
func stringSetFromSlice(ctx context.Context, in []string) types.Set {
	if in == nil {
		return types.SetNull(types.StringType)
	}
	out, diags := types.SetValueFrom(ctx, types.StringType, in)
	if diags.HasError() {
		// The element type is String; conversion of []string cannot fail.
		// Fall back to null rather than panic in a mapping helper.
		return types.SetNull(types.StringType)
	}
	return out
}
