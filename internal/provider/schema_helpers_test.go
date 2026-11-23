package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestUseNonNullStateForUnknown(t *testing.T) {
	someState := tfsdk.State{Raw: tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})}
	cases := []struct {
		name  string
		state types.Bool
		want  types.Bool
	}{
		{"known prior is kept", types.BoolValue(true), types.BoolValue(true)},
		{"null prior stays unknown", types.BoolNull(), types.BoolUnknown()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := planmodifier.BoolRequest{
				State: someState, StateValue: c.state,
				PlanValue: types.BoolUnknown(), ConfigValue: types.BoolNull(),
			}
			resp := &planmodifier.BoolResponse{PlanValue: req.PlanValue}
			useNonNullStateForUnknown{}.PlanModifyBool(context.Background(), req, resp)
			if !resp.PlanValue.Equal(c.want) {
				t.Errorf("planned %v, want %v", resp.PlanValue, c.want)
			}
		})
	}
}

// The generator gives the modifier to each bool that reads null for a refused
// value and has a default, on a resource that updates with a PUT, and to no other.
func TestNullRefusedBoolsGetTheModifier(t *testing.T) {
	cases := []struct {
		r    resource.Resource
		attr string
		want bool
	}{
		{&firewallRuleResource{}, "enabled", true},
		{&firewallZoneResource{}, "masq", true},
		{&firewallRedirectResource{}, "reflection", false},
		{&firewallDefaultsResource{}, "drop_invalid", false},
	}
	for _, c := range cases {
		resp := &resource.SchemaResponse{}
		c.r.Schema(context.Background(), resource.SchemaRequest{}, resp)
		a, ok := resp.Schema.Attributes[c.attr].(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%T.%s is not a bool attribute", c.r, c.attr)
		}
		got := false
		for _, m := range a.PlanModifiers {
			if _, ok := m.(useNonNullStateForUnknown); ok {
				got = true
			}
		}
		if got != c.want {
			t.Errorf("%T.%s has the modifier = %v, want %v", c.r, c.attr, got, c.want)
		}
	}
}
