package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

	// uapi 3.0 answers null for an absent uci list, and unset must stay
	// distinguishable from set-to-empty, so an absent key becomes a null list.
	mv, _ := listVal(ctx, m, "missing")
	if !mv.IsNull() {
		t.Errorf("absent list should be null, got %v", mv)
	}
	// An explicit null is the same case.
	nv, _ := listVal(ctx, map[string]any{"k": nil}, "k")
	if !nv.IsNull() {
		t.Errorf("null list should be null, got %v", nv)
	}
	// An empty array is not absent: it stays an empty, non-null list.
	ev, _ := listVal(ctx, map[string]any{"k": []any{}}, "k")
	if ev.IsNull() || len(ev.Elements()) != 0 {
		t.Errorf("empty array should be an empty list, got %v", ev)
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

func TestListValKeepEmpty(t *testing.T) {
	ctx := context.Background()
	empty := types.ListValueMust(types.StringType, []attr.Value{})
	one := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a")})
	cases := []struct {
		name  string
		obj   map[string]any
		prior types.List
		want  types.List
	}{
		{"absent keeps a configured empty list", map[string]any{}, empty, empty},
		{"null keeps a configured empty list", map[string]any{"k": nil}, empty, empty},
		{"absent with null prior stays null", map[string]any{}, types.ListNull(types.StringType), types.ListNull(types.StringType)},
		{"absent with unknown prior is null", map[string]any{}, types.ListUnknown(types.StringType), types.ListNull(types.StringType)},
		{"absent does not keep a non-empty prior", map[string]any{}, one, types.ListNull(types.StringType)},
		{"a server value wins over an empty prior", map[string]any{"k": []any{"a"}}, empty, one},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, d := listValKeepEmpty(ctx, c.obj, "k", c.prior)
			if d.HasError() {
				t.Fatal(d)
			}
			if !got.Equal(c.want) {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}
