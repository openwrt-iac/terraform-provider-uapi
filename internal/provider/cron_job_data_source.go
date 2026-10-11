package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &cronJobDataSource{}
	_ datasource.DataSourceWithConfigure = &cronJobDataSource{}
)

type cronJobDataSource struct{ client *client.Client }

func NewCronJobDataSource() datasource.DataSource { return &cronJobDataSource{} }

type cronJobRuntimeModel struct {
	CrondRunning  types.Bool `tfsdk:"crond_running"`
	RunsAsWritten types.Bool `tfsdk:"runs_as_written"`
}

type cronJobDataSourceModel struct {
	ID       types.String         `tfsdk:"id"`
	Schedule types.String         `tfsdk:"schedule"`
	Command  types.String         `tfsdk:"command"`
	Runtime  *cronJobRuntimeModel `tfsdk:"runtime"`
}

func (d *cronJobDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cron_job"
}

func (d *cronJobDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *cronJobDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up a root crontab entry by id, as crond reads it.",
		Attributes: map[string]dsschema.Attribute{
			"id":       dsIDAttribute(),
			"schedule": dsComputedString("The five time fields as crond reads them, joined by single spaces."),
			"command":  dsComputedString("The command as crond runs it: lines ending in `\\` joined to the next, cut at the first `#`, surrounding whitespace stripped."),
			"runtime": dsschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Live crond state (read-only).",
				Attributes: map[string]dsschema.Attribute{
					"crond_running":   dsComputedBool("Whether procd reports a running crond. False means no entry runs, whatever the file says: the init script does not start crond while `/etc/crontabs` is empty, and stops it when `system.cronloglevel` is invalid. Null when procd could not be asked."),
					"runs_as_written": dsComputedBool("False when crond runs this schedule differently from how it reads, such as minute 75 (every minute), weekday 7 (every day) or `@daily` (never). uapi refuses such a schedule on write, so only another writer can leave one."),
				},
			},
		},
	}
}

func (d *cronJobDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m cronJobDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := d.client.GetObject(ctx, "/"+cronJobCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading cron job", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Cron job not found", "No cron job with id "+m.ID.ValueString())
		return
	}
	m.ID = strVal(obj, "id")
	m.Schedule = strVal(obj, "schedule")
	m.Command = strVal(obj, "command")
	rt, _ := obj["runtime"].(map[string]any)
	m.Runtime = &cronJobRuntimeModel{
		CrondRunning:  boolVal(rt, "crond_running"),
		RunsAsWritten: boolVal(rt, "runs_as_written"),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
