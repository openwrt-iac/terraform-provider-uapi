package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &diagnosticsDataSource{}
	_ datasource.DataSourceWithConfigure = &diagnosticsDataSource{}
)

type diagnosticsDataSource struct{ client *client.Client }

func NewDiagnosticsDataSource() datasource.DataSource { return &diagnosticsDataSource{} }

type diagnosticsModel struct {
	Validate          types.Bool              `tfsdk:"validate"`
	Version           types.String            `tfsdk:"version"`
	UptimeSeconds     types.Int64             `tfsdk:"uptime_seconds"`
	ResourcesLoaded   types.List              `tfsdk:"resources_loaded"`
	LockState         *lockStateModel         `tfsdk:"lock_state"`
	RecentErrors      []recentErrorModel      `tfsdk:"recent_errors"`
	ManagementPath    *managementPathModel    `tfsdk:"management_path"`
	InvalidSections   []invalidSectionModel   `tfsdk:"invalid_sections"`
	SweptResources    types.List              `tfsdk:"swept_resources"`
	SkippedForScope   types.List              `tfsdk:"skipped_for_scope"`
	RequestID         types.String            `tfsdk:"request_id"`
	StateDirs         types.Map               `tfsdk:"state_dirs"`
	Limits            *limitsModel            `tfsdk:"limits"`
	InterruptedWrites []interruptedWriteModel `tfsdk:"interrupted_writes"`
}

type limitsModel struct {
	MaxBodyBytes      types.Int64 `tfsdk:"max_body_bytes"`
	MaxBodyContainers types.Int64 `tfsdk:"max_body_containers"`
	BodyReadS         types.Int64 `tfsdk:"body_read_s"`
	ReloadBudgetS     types.Int64 `tfsdk:"reload_budget_s"`
}

type interruptedWriteModel struct {
	PID           types.Int64  `tfsdk:"pid"`
	RequestID     types.String `tfsdk:"request_id"`
	Packages      types.List   `tfsdk:"packages"`
	Staged        types.List   `tfsdk:"staged"`
	StartedAt     types.Int64  `tfsdk:"started_at"`
	ReloadStarted types.Bool   `tfsdk:"reload_started"`
}

type managementPathModel struct {
	Address   types.String `tfsdk:"address"`
	Device    types.String `tfsdk:"device"`
	Interface types.String `tfsdk:"interface"`
}

type invalidSectionModel struct {
	Resource types.String            `tfsdk:"resource"`
	ID       types.String            `tfsdk:"id"`
	Managed  types.Bool              `tfsdk:"managed"`
	Errors   []invalidSectionErrItem `tfsdk:"errors"`
}

type invalidSectionErrItem struct {
	Field   types.String `tfsdk:"field"`
	Code    types.String `tfsdk:"code"`
	Message types.String `tfsdk:"message"`
}

type lockStateModel struct {
	GlobalHeld   types.Bool `tfsdk:"global_held"`
	PackagesHeld types.List `tfsdk:"packages_held"`
}

type recentErrorModel struct {
	Ts        types.Int64  `tfsdk:"ts"`
	RequestID types.String `tfsdk:"request_id"`
	Code      types.String `tfsdk:"code"`
	Status    types.Int64  `tfsdk:"status"`
	Method    types.String `tfsdk:"method"`
	Path      types.String `tfsdk:"path"`
	Message   types.String `tfsdk:"message"`
}

func (d *diagnosticsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_diagnostics"
}

func (d *diagnosticsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *diagnosticsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Runtime diagnostics: loaded resources, uptime, lock state, and an optional sweep for sections a write would reject.",
		Attributes: map[string]dsschema.Attribute{
			"validate": dsschema.BoolAttribute{
				Optional: true,
				Description: "Run the validation sweep (`?validate=1`), populating `invalid_sections`, `swept_resources` and " +
					"`skipped_for_scope`. Off by default: the sweep re-validates every section on the router, so it costs " +
					"real work on each read.",
			},
			"version":          dsComputedString("uapi version string."),
			"uptime_seconds":   dsComputedInt64("Seconds since the router booted."),
			"resources_loaded": dsComputedStringList("Curated resource keys the server has loaded."),
			"lock_state": dsschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Current advisory lock holders.",
				Attributes: map[string]dsschema.Attribute{
					"global_held":   dsComputedBool("Whether the global uapi lock is held."),
					"packages_held": dsComputedStringList("Packages whose per-package lock is held."),
				},
			},
			"recent_errors": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "Best-effort sliding window of recent error responses (newest last).",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"ts":         dsComputedInt64("Unix epoch seconds when the error occurred."),
						"request_id": dsComputedString("Request id of the failed request."),
						"code":       dsComputedString("Error code."),
						"status":     dsComputedInt64("HTTP status."),
						"method":     dsComputedString("HTTP method."),
						"path":       dsComputedString("Request path."),
						"message":    dsComputedString("Error message."),
					},
				},
			},
			"management_path": dsschema.SingleNestedAttribute{
				Computed: true,
				Description: "Which interface this request arrived through. Absent when the inbound address or its route " +
					"could not be determined.",
				Attributes: map[string]dsschema.Attribute{
					"address":   dsComputedString("Inbound address the request arrived on."),
					"device":    dsComputedString("Kernel device carrying that address."),
					"interface": dsComputedString("uci network interface owning the device, when one claims it."),
				},
			},
			"invalid_sections": dsschema.ListNestedAttribute{
				Computed: true,
				Description: "Sections a write would reject today, whether or not Terraform manages them. Populated only " +
					"when `validate` is set. An empty list next to a non-empty `skipped_for_scope` means the token was " +
					"not allowed to look, not that nothing is wrong.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"resource": dsComputedString("Resource in slash form, e.g. `firewall/rules`."),
						"id":       dsComputedString("Section id."),
						"managed":  dsComputedBool("Whether the underlying uci section is uapi-managed."),
						"errors": dsschema.ListNestedAttribute{
							Computed:    true,
							Description: "Why the section would be rejected.",
							NestedObject: dsschema.NestedAttributeObject{
								Attributes: map[string]dsschema.Attribute{
									"field":   dsComputedString("Offending field."),
									"code":    dsComputedString("Error code."),
									"message": dsComputedString("Human-readable explanation."),
								},
							},
						},
					},
				},
			},
			"swept_resources":   dsComputedStringList("Resources the sweep checked, in colon form. Populated only when `validate` is set."),
			"skipped_for_scope": dsComputedStringList("Resources left out because the token lacks `:ro` on them. Populated only when `validate` is set."),
			"request_id":        dsComputedString("Request id assigned to this diagnostics call."),
			"state_dirs": dsschema.MapAttribute{
				ElementType: types.BoolType,
				Computed:    true,
				Description: "State directories under /var/run, keyed by path, as uapi finds them; the read creates " +
					"and changes nothing. `true`: a directory owned by uapi's request uid, mode 0700, which uapi uses. " +
					"`false`: the path holds anything else, and uapi does not use it as it stands. `null`: the path does " +
					"not exist yet, and uapi creates it at its first write there. Needs uapi 3.3.0 or later; reads as " +
					"null against an older server.",
			},
			"limits": dsschema.SingleNestedAttribute{
				Computed: true,
				Description: "The request bounds that uapi applies: the `config limits` options of /etc/config/uapi, with the " +
					"compiled default where an option is absent or out of bounds. An option can lower a bound, not raise it. " +
					"Needs uapi 3.5.0 or later; reads as null against an older server.",
				Attributes: map[string]dsschema.Attribute{
					"max_body_bytes":      dsComputedInt64("Largest request body in bytes. A larger `Content-Length` answers 413."),
					"max_body_containers": dsComputedInt64("Largest number of `{` and `[` characters in a request body, which bounds the JSON parse."),
					"body_read_s":         dsComputedInt64("Seconds that a request body has to arrive in full."),
					"reload_budget_s":     dsComputedInt64("Seconds that a write waits in total for its daemon reloads. After that, the write answers `X-Reload-Status: unverified`."),
				},
			},
			"interrupted_writes": dsschema.ListNestedAttribute{
				Computed: true,
				Description: "Writes whose request process stopped after the commit and before the write finished: uhttpd " +
					"killed it (a client disconnect or `script_timeout`), or the write threw. The commit can be in uci with no " +
					"check, no restore and no answer to the client. Report only: uapi corrects nothing, and the records stay " +
					"until a reboot. Empty when no write was interrupted. Needs uapi 3.5.0 or later; reads as null against an " +
					"older server.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"pid":            dsComputedInt64("Process id of the request that made the write."),
						"request_id":     dsComputedString("Request id of the write, when it had one."),
						"packages":       dsComputedStringList("uci packages of the write. Empty for a write outside uci."),
						"staged":         dsComputedStringList("Names of the sections that the package held with the staged changes, just before the commit. Null for a write over several packages or outside uci."),
						"started_at":     dsComputedInt64("Unix epoch seconds when uapi recorded the write, just before the commit."),
						"reload_started": dsComputedBool("Whether the daemon reload of the write had started."),
					},
				},
			},
		},
	}
}

func (d *diagnosticsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg diagnosticsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	path := "/diagnostics"
	if cfg.Validate.ValueBool() {
		path += "?validate=1"
	}
	obj, _, found, err := d.client.GetObject(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("Error reading diagnostics", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Error reading diagnostics", "endpoint returned not found")
		return
	}
	loaded, ld := listVal(ctx, obj, "resources_loaded")
	resp.Diagnostics.Append(ld...)
	swept, sd := listVal(ctx, obj, "swept_resources")
	resp.Diagnostics.Append(sd...)
	skipped, kd := listVal(ctx, obj, "skipped_for_scope")
	resp.Diagnostics.Append(kd...)
	out := diagnosticsModel{
		Validate:        cfg.Validate,
		Version:         strVal(obj, "version"),
		UptimeSeconds:   int64Val(obj, "uptime_seconds"),
		ResourcesLoaded: loaded,
		SweptResources:  swept,
		SkippedForScope: skipped,
		RequestID:       strVal(obj, "request_id"),
		// Explicitly null: a server older than uapi 3.3.0 answers no
		// state_dirs, and a zero MapValue carries no element type, which
		// the state encode rejects as a hard provider error.
		StateDirs: types.MapNull(types.BoolType),
	}
	if mp, ok := obj["management_path"].(map[string]any); ok {
		out.ManagementPath = &managementPathModel{
			Address:   strVal(mp, "address"),
			Device:    strVal(mp, "device"),
			Interface: strVal(mp, "interface"),
		}
	}
	// Seeded only when the sweep ran. A nil slice reflects to a null attribute, and
	// null has to keep meaning "not looked at": as of uapi 3.0 listVal leaves the two
	// sibling lists null when absent, which is exactly what a read without validate
	// returns. With the sweep on, all three are non-null so length() and for_each
	// work on a clean result.
	if cfg.Validate.ValueBool() {
		out.InvalidSections = []invalidSectionModel{}
	}
	if arr, ok := obj["invalid_sections"].([]any); ok {
		for _, e := range arr {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			sec := invalidSectionModel{
				Resource: strVal(m, "resource"),
				ID:       strVal(m, "id"),
				Managed:  boolVal(m, "managed"),
			}
			if errs, ok := m["errors"].([]any); ok {
				for _, ee := range errs {
					em, ok := ee.(map[string]any)
					if !ok {
						continue
					}
					sec.Errors = append(sec.Errors, invalidSectionErrItem{
						Field:   strVal(em, "field"),
						Code:    strVal(em, "code"),
						Message: strVal(em, "message"),
					})
				}
			}
			out.InvalidSections = append(out.InvalidSections, sec)
		}
	}
	if ls, ok := obj["lock_state"].(map[string]any); ok {
		names := []string{}
		if pp, ok := ls["per_package"].(map[string]any); ok {
			for k := range pp {
				names = append(names, k)
			}
			sort.Strings(names)
		}
		held, hd := types.ListValueFrom(ctx, types.StringType, names)
		resp.Diagnostics.Append(hd...)
		out.LockState = &lockStateModel{
			GlobalHeld:   boolValDefault(ls, "global_held"),
			PackagesHeld: held,
		}
	}
	if arr, ok := obj["recent_errors"].([]any); ok {
		for _, e := range arr {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			out.RecentErrors = append(out.RecentErrors, recentErrorModel{
				Ts:        int64Val(m, "ts"),
				RequestID: strVal(m, "request_id"),
				Code:      strVal(m, "code"),
				Status:    int64Val(m, "status"),
				Method:    strVal(m, "method"),
				Path:      strVal(m, "path"),
				Message:   strVal(m, "message"),
			})
		}
	}
	// A state_dirs value may be null: a directory that does not exist yet
	// reads null, and the map element carries it instead of collapsing.
	if sd, ok := obj["state_dirs"].(map[string]any); ok {
		vals := map[string]attr.Value{}
		for k, v := range sd {
			if v == nil {
				vals[k] = types.BoolNull()
				continue
			}
			b, _ := v.(bool)
			vals[k] = types.BoolValue(b)
		}
		m, md := types.MapValue(types.BoolType, vals)
		resp.Diagnostics.Append(md...)
		out.StateDirs = m
	}
	if lm, ok := obj["limits"].(map[string]any); ok {
		out.Limits = &limitsModel{
			MaxBodyBytes:      int64Val(lm, "max_body_bytes"),
			MaxBodyContainers: int64Val(lm, "max_body_containers"),
			BodyReadS:         int64Val(lm, "body_read_s"),
			ReloadBudgetS:     int64Val(lm, "reload_budget_s"),
		}
	}
	// Non-nil once the key is there, so a server with nothing to report reads
	// an empty list, and only an older server, which sends no key, reads null.
	if arr, ok := obj["interrupted_writes"].([]any); ok {
		out.InterruptedWrites = []interruptedWriteModel{}
		for _, e := range arr {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			pkgs, pd := listVal(ctx, m, "packages")
			resp.Diagnostics.Append(pd...)
			staged, sd := listVal(ctx, m, "staged")
			resp.Diagnostics.Append(sd...)
			out.InterruptedWrites = append(out.InterruptedWrites, interruptedWriteModel{
				PID:           int64Val(m, "pid"),
				RequestID:     strVal(m, "request_id"),
				Packages:      pkgs,
				Staged:        staged,
				StartedAt:     int64Val(m, "started_at"),
				ReloadStarted: boolVal(m, "reload_started"),
			})
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
