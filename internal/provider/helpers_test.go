package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStrVal(t *testing.T) {
	m := map[string]any{"a": "x", "b": nil}
	if v := strVal(m, "a"); v.ValueString() != "x" {
		t.Errorf("a = %v", v)
	}
	if v := strVal(m, "b"); !v.IsNull() {
		t.Errorf("nil should be null, got %v", v)
	}
	if v := strVal(m, "missing"); !v.IsNull() {
		t.Errorf("missing should be null, got %v", v)
	}
}

func TestBoolVal(t *testing.T) {
	m := map[string]any{"t": true, "f": false, "n": nil}
	if v := boolVal(m, "t"); v.ValueBool() != true {
		t.Errorf("t = %v", v)
	}
	if v := boolVal(m, "f"); v.ValueBool() != false {
		t.Errorf("f = %v", v)
	}
	if v := boolVal(m, "n"); !v.IsNull() {
		t.Errorf("n should be null")
	}
}

func TestInt64Val(t *testing.T) {
	m := map[string]any{"n": float64(1700000000)}
	if v := int64Val(m, "n"); v.ValueInt64() != 1700000000 {
		t.Errorf("n = %v", v)
	}
}

func TestListVal(t *testing.T) {
	ctx := context.Background()
	m := map[string]any{"l": []any{"a", "b"}, "empty": []any{}}

	v, diags := listVal(ctx, m, "l")
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	var out []string
	v.ElementsAs(ctx, &out, false)
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Errorf("list = %v", out)
	}

	// Missing key becomes an empty (non-null) list to match the API.
	mv, _ := listVal(ctx, m, "missing")
	if mv.IsNull() {
		t.Error("missing list should be empty, not null")
	}
	if len(mv.Elements()) != 0 {
		t.Errorf("missing list should be empty, got %v", mv.Elements())
	}
}

func TestPutHelpers(t *testing.T) {
	ctx := context.Background()
	out := map[string]any{}

	putStr(out, "name", types.StringValue("foo"))
	putStr(out, "skip_null", types.StringNull())
	putStr(out, "skip_unknown", types.StringUnknown())
	putBool(out, "flag", types.BoolValue(true))
	putBool(out, "skip_bool", types.BoolNull())

	list, _ := types.ListValueFrom(ctx, types.StringType, []string{"x"})
	var diags diag.Diagnostics
	putList(ctx, out, "items", list, &diags)
	putList(ctx, out, "skip_list", types.ListNull(types.StringType), &diags)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}

	if out["name"] != "foo" {
		t.Errorf("name = %v", out["name"])
	}
	if _, ok := out["skip_null"]; ok {
		t.Error("null string should be omitted")
	}
	if _, ok := out["skip_unknown"]; ok {
		t.Error("unknown string should be omitted")
	}
	if out["flag"] != true {
		t.Errorf("flag = %v", out["flag"])
	}
	if _, ok := out["skip_bool"]; ok {
		t.Error("null bool should be omitted")
	}
	items, ok := out["items"].([]string)
	if !ok || len(items) != 1 || items[0] != "x" {
		t.Errorf("items = %v", out["items"])
	}
	if _, ok := out["skip_list"]; ok {
		t.Error("null list should be omitted")
	}
}
