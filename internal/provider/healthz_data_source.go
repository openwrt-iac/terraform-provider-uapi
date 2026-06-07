package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &healthzDataSource{}
	_ datasource.DataSourceWithConfigure = &healthzDataSource{}
)

type healthzDataSource struct{ client *client.Client }

func NewHealthzDataSource() datasource.DataSource { return &healthzDataSource{} }

type healthzModel struct {
	Status  types.String       `tfsdk:"status"`
	Version types.String       `tfsdk:"version"`
	Checks  *healthzCheckModel `tfsdk:"checks"`
	Errors  types.List         `tfsdk:"errors"`
}

type healthzCheckModel struct {
	Ubus     types.String `tfsdk:"ubus"`
	UCI      types.String `tfsdk:"uci"`
	LockDir  types.String `tfsdk:"lock_dir"`
	TimeSync types.String `tfsdk:"time_sync"`
}

func (d *healthzDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_healthz"
}

func (d *healthzDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *healthzDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Liveness and readiness of the uapi service and its dependencies.",
		Attributes: map[string]dsschema.Attribute{
			"status":  dsComputedString("Overall status: ok or degraded."),
			"version": dsComputedString("uapi version string."),
			"checks": dsschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Per-dependency health (ok, degraded, or unknown).",
				Attributes: map[string]dsschema.Attribute{
					"ubus":      dsComputedString("ubus connectivity."),
					"uci":       dsComputedString("uci read access."),
					"lock_dir":  dsComputedString("Presence of the lock directory."),
					"time_sync": dsComputedString("Clock synchronization state."),
				},
			},
			"errors": dsComputedStringList("Human-readable details for any degraded check."),
		},
	}
}

func (d *healthzDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	obj, _, found, err := d.client.GetObject(ctx, "/healthz")
	if err != nil {
		resp.Diagnostics.AddError("Error reading healthz", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Error reading healthz", "endpoint returned not found")
		return
	}
	errs, ed := listVal(ctx, obj, "errors")
	resp.Diagnostics.Append(ed...)
	out := healthzModel{
		Status:  strVal(obj, "status"),
		Version: strVal(obj, "version"),
		Errors:  errs,
	}
	if c, ok := obj["checks"].(map[string]any); ok {
		out.Checks = &healthzCheckModel{
			Ubus:     strVal(c, "ubus"),
			UCI:      strVal(c, "uci"),
			LockDir:  strVal(c, "lock_dir"),
			TimeSync: strVal(c, "time_sync"),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
