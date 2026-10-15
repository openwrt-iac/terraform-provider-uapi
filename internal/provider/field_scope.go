package provider

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// fieldScope is a generated table, from the spec's `allOf` entries, of the
// fields that some values of one discriminator attribute do not take, such as
// the network_interface fields that a proto does not take. uapi answers 422 to
// a value for such a field and leaves it out of a read.
type fieldScope struct {
	Field string
	Rules []scopeRule
}

// scopeRule: when Field is one of Values, each of Fields is null.
type scopeRule struct {
	Values []string
	Fields []string
}

// excluded returns the fields that value v does not take. A value that no rule
// names takes every field.
func (s fieldScope) excluded(v string) []string {
	var out []string
	for _, r := range s.Rules {
		if slices.Contains(r.Values, v) {
			out = append(out, r.Fields...)
		}
	}
	return out
}

func (s fieldScope) scoped() []string {
	var out []string
	for _, r := range s.Rules {
		out = append(out, r.Fields...)
	}
	return out
}

// validateConfig reports at plan time the value that uapi refuses at apply. An
// empty list passes, as uapi stores nothing for it and listValKeepEmpty keeps
// it. An empty string does not pass: it reads back null, and the apply would
// fail as an inconsistent result.
func (s fieldScope) validateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var v types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(s.Field), &v)...)
	if resp.Diagnostics.HasError() || v.IsNull() || v.IsUnknown() {
		return
	}
	for _, f := range s.excluded(v.ValueString()) {
		var cfg attr.Value
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(f), &cfg)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if cfg.IsNull() || cfg.IsUnknown() {
			continue
		}
		if l, ok := cfg.(types.List); ok && len(l.Elements()) == 0 {
			continue
		}
		resp.Diagnostics.AddAttributeError(path.Root(f), "Attribute does not apply to this "+s.Field,
			fmt.Sprintf("%s %q does not take %s, and uapi refuses a value for it. Remove %s from the configuration, or set it to null.", s.Field, v.ValueString(), f, f))
	}
}

// modifyPlan sets to null each field that the planned value does not take and
// that the configuration does not set. Without it, an Optional+Computed field
// keeps its prior value through UseStateForUnknown, so a proto switch sends the
// fields of the old proto, and uapi refuses the PUT. When the planned value is
// unknown, these fields become unknown: no prior value is safe to send.
func (s fieldScope) modifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var v, prior types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(s.Field), &v)...)
	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(s.Field), &prior)...)
	}
	if resp.Diagnostics.HasError() || v.IsNull() {
		return
	}
	if v.IsUnknown() {
		s.plan(ctx, req, resp, s.scoped(), tftypes.UnknownValue)
		return
	}
	excluded := s.excluded(v.ValueString())
	s.plan(ctx, req, resp, excluded, nil)
	if prior.IsNull() || prior.Equal(v) {
		return
	}
	// A field that the old proto did not take read null, and UseStateForUnknown
	// copies a null too. The new proto can read a default there, such as
	// nohostroute false on wireguard.
	entering := slices.DeleteFunc(s.excluded(prior.ValueString()), func(f string) bool {
		return slices.Contains(excluded, f)
	})
	s.plan(ctx, req, resp, entering, tftypes.UnknownValue)
}

// plan sets each Computed field of fields that the configuration does not set
// to raw, null or unknown. A field that is not Computed plans null with a null
// configuration, and must keep that plan.
func (s fieldScope) plan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, fields []string, raw any) {
	for _, f := range fields {
		p := path.Root(f)
		a, d := req.Plan.Schema.AttributeAtPath(ctx, p)
		resp.Diagnostics.Append(d...)
		var cfg, planned attr.Value
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, p, &cfg)...)
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, p, &planned)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !a.IsComputed() || !cfg.IsNull() {
			continue
		}
		t := planned.Type(ctx)
		nv, err := t.ValueFromTerraform(ctx, tftypes.NewValue(t.TerraformType(ctx), raw))
		if err != nil {
			resp.Diagnostics.AddAttributeError(p, "Error planning "+f, err.Error())
			return
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, p, nv)...)
	}
}
