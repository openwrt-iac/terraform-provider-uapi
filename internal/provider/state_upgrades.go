package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// State upgraders for the attributes whose Terraform type changed shape in a way
// Terraform's own decoder cannot absorb. `plan` stops before producing anything,
// naming a schema mismatch rather than the field that moved (issue #28).
//
// "Cannot absorb" is the test that matters, and it is narrower than "the type
// changed". The passthrough decoder coerces between JSON scalars: a stored `true`
// reads back into a string attribute as "true", and "64" into a number. Only the
// list/scalar boundary actually fails, which is why `firewall_redirect` (list ->
// string) and `dhcp_host.tag` (string -> list) need upgraders and the two
// bool -> string corrections in 2.5.0 did not. Measured against
// tfprotov6.RawState.Unmarshal rather than assumed.
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
func firstOfList(m map[string]any, key string) (dropped int) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	list, ok := v.([]any)
	if !ok {
		return 0
	}
	if len(list) == 0 {
		delete(m, key)
		return 0
	}
	m[key] = list[0]
	return len(list) - 1
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
						// A 2.x state could hold more than one entry even though
						// firewall4 discarded such a section: say so rather than
						// truncating in silence.
						if n := firstOfList(match, k); n > 0 {
							resp.Diagnostics.AddWarning(
								"Dropped extra values while upgrading state",
								fmt.Sprintf("match.%s held %d values and is a single value as of 3.0.0, so %d were dropped. "+
									"firewall4 discarded any redirect writing a uci list, so they were never in effect on the router.", k, n+1, n))
						}
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
					// uci stores multiple tags as one space-separated option, and
					// uapi answers with the split list, so the upgrade has to split
					// too: wrapping the whole string would produce one bogus tag.
					parts := strings.Fields(s)
					if len(parts) == 0 {
						delete(raw, "tag")
					} else {
						out := make([]any, len(parts))
						for i, p := range parts {
							out[i] = p
						}
						raw["tag"] = out
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
