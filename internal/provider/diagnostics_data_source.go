package provider

import (
	"context"
	"sort"

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
	Version         types.String       `tfsdk:"version"`
	UptimeSeconds   types.Int64        `tfsdk:"uptime_seconds"`
	ResourcesLoaded types.List         `tfsdk:"resources_loaded"`
	LockState       *lockStateModel    `tfsdk:"lock_state"`
	RecentErrors    []recentErrorModel `tfsdk:"recent_errors"`
	RequestID       types.String       `tfsdk:"request_id"`
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
		Description: "Runtime diagnostics: loaded resources, uptime, and lock state.",
		Attributes: map[string]dsschema.Attribute{
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
			"request_id": dsComputedString("Request id assigned to this diagnostics call."),
		},
	}
}

func (d *diagnosticsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	obj, _, found, err := d.client.GetObject(ctx, "/diagnostics")
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
	out := diagnosticsModel{
		Version:         strVal(obj, "version"),
		UptimeSeconds:   int64Val(obj, "uptime_seconds"),
		ResourcesLoaded: loaded,
		RequestID:       strVal(obj, "request_id"),
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
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
