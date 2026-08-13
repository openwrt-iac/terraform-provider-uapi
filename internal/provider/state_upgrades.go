package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// State upgraders for the two attributes whose Terraform type changed shape. A
// type change without a schema version bump is not a diff, it is an undecodable
// state: `plan` stops before producing anything, naming a schema mismatch rather
// than the field that moved (issue #28).
//
// Version 0 is ambiguous, which is what these are written around. The provider
// never set a version before 3.0.1, so state stamped 0 may hold either shape:
// written by 2.x, or by the release that changed the type and shipped without a
// bump. Terraform runs the 0 -> 1 upgrader over both, so each one normalizes
// rather than converts, and running it twice changes nothing.
//
// They read req.RawState.JSON rather than declaring a PriorSchema for that same
// reason: a PriorSchema fixes exactly one prior shape and fails to decode the
// other, which would trade a break for 2.x users for a break for 3.0.0 ones.
// The normalized map is API-shaped, which is what each resource's own read()
// already consumes, so the conversion to model values stays in one place.

// rawPriorState decodes prior state as untyped JSON. The upgraders normalize the
// map in place before handing it to read().
func rawPriorState(req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) (map[string]any, bool) {
	if req.RawState == nil {
		resp.Diagnostics.AddError("Missing prior state", "Terraform sent no prior state to upgrade.")
		return nil, false
	}
	var raw map[string]any
	if err := json.Unmarshal(req.RawState.JSON, &raw); err != nil {
		resp.Diagnostics.AddError("Could not read prior state",
			"The saved state is not valid JSON, so it cannot be upgraded to the current schema.\n\n"+err.Error())
		return nil, false
	}
	return raw, true
}

// firstOfList collapses a list-valued key to its first element. Lossless here:
// firewall4 parses a `config redirect` option as a scalar and discards any
// section that writes a uci list, so a second entry never reached the router.
// A scalar (state written after the type change) is left alone.
func firstOfList(m map[string]any, key string) {
	v, ok := m[key]
	if !ok || v == nil {
		return
	}
	list, ok := v.([]any)
	if !ok {
		return
	}
	if len(list) == 0 {
		delete(m, key)
		return
	}
	m[key] = list[0]
}

// redirectMatchScalars are the match selectors that became scalars. `proto` is
// deliberately absent: it stays a list.
var redirectMatchScalars = []string{"src_ip", "src_port", "src_dip", "src_dport", "dest_ip", "dest_port"}

func (r *firewallRedirectResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				raw, ok := rawPriorState(req, resp)
				if !ok {
					return
				}
				if match, isObj := raw["match"].(map[string]any); isObj {
					for _, k := range redirectMatchScalars {
						firstOfList(match, k)
					}
				}
				var m firewallRedirectModel
				ds := newDiagsink(&resp.Diagnostics)
				r.read(ctx, raw, &m, ds)
				// read() does not touch etag: a write takes it from the response
				// header. Prior state has it, and nothing is copied across
				// automatically, so it is carried over by hand.
				m.ETag = strVal(raw, "etag")
				resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			},
		},
	}
}

// dropIfBool removes a key whose prior state holds a boolean. Used for the two
// attributes uapi 2.5.0 corrected from boolean to string, where the old value is
// not convertible: `urandom_seed` is the path the seed is saved to (/sbin/
// urandom_seed tests that it starts with "/"), and `lldp_description` is the
// description advertised in LLDP frames. A stored `true` never encoded either, so
// mapping it to "1" or "true" would invent a value the router never had. Both
// attributes are Optional+Computed, so dropping to null is legitimate and the
// refresh that precedes the next plan fills in what the router actually holds.
func dropIfBool(m map[string]any, key string) {
	if _, isBool := m[key].(bool); isBool {
		delete(m, key)
	}
}

func (r *systemResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				raw, ok := rawPriorState(req, resp)
				if !ok {
					return
				}
				dropIfBool(raw, "urandom_seed")
				var m systemModel
				ds := newDiagsink(&resp.Diagnostics)
				r.read(ctx, raw, &m, ds)
				m.ETag = strVal(raw, "etag")
				resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			},
		},
	}
}

func (r *lldpdConfigResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				raw, ok := rawPriorState(req, resp)
				if !ok {
					return
				}
				dropIfBool(raw, "lldp_description")
				var m lldpdConfigModel
				ds := newDiagsink(&resp.Diagnostics)
				r.read(ctx, raw, &m, ds)
				m.ETag = strVal(raw, "etag")
				resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			},
		},
	}
}

func (r *dhcpHostResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				raw, ok := rawPriorState(req, resp)
				if !ok {
					return
				}
				// The opposite direction, and a release earlier: `tag` was a
				// string through 2.4.x and became a list in 2.5.0, which also
				// shipped without a version bump.
				if s, isString := raw["tag"].(string); isString {
					if s == "" {
						delete(raw, "tag")
					} else {
						raw["tag"] = []any{s}
					}
				}
				var m dhcpHostModel
				ds := newDiagsink(&resp.Diagnostics)
				r.read(ctx, raw, &m, ds)
				m.ETag = strVal(raw, "etag")
				resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			},
		},
	}
}
