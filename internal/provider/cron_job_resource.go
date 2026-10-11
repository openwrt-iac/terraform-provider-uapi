package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

const cronJobCollection = "system/cron_jobs"

const (
	cronJobIDDesc       = "Stable id: sha256 prefix of `<schedule> <command>`. A change to either gives a new id."
	cronJobScheduleDesc = "Five fields separated by single spaces: minute 0-59, hour 0-23, day 1-31, month 1-12 or jan-dec, weekday 0-6 or sun-sat. Each takes lists, ranges and `*`, and a `/step` after `*` or a range. uapi refuses what OpenWrt's busybox crond does not run as written: weekday 7, `@daily`-style schedules, and `*/N` in the day field (write `1-31/N`)."
	cronJobCommandDesc  = "Shell command run as root. Must not contain `#` or control characters, start or end with whitespace, or end with `\\`: crond would cut, strip or join them, and the entry would read back different."
)

var (
	_ resource.Resource                = &cronJobResource{}
	_ resource.ResourceWithConfigure   = &cronJobResource{}
	_ resource.ResourceWithImportState = &cronJobResource{}
)

type cronJobResource struct {
	client *client.Client
}

func NewCronJobResource() resource.Resource {
	return &cronJobResource{}
}

type cronJobModel struct {
	ID       types.String `tfsdk:"id"`
	Schedule types.String `tfsdk:"schedule"`
	Command  types.String `tfsdk:"command"`
}

func (r *cronJobResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cron_job"
}

func (r *cronJobResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *cronJobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "One line of the root crontab (`/etc/crontabs/root`). Every other line stays as it is, so the lines that packages add survive. There is no per-job update: changing the schedule or the command replaces the job.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   cronJobIDDesc,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"schedule": schema.StringAttribute{
				Required:      true,
				Description:   cronJobScheduleDesc,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"command": schema.StringAttribute{
				Required:      true,
				Description:   cronJobCommandDesc,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *cronJobResource) read(obj map[string]any, m *cronJobModel) {
	m.ID = strVal(obj, "id")
	m.Schedule = strVal(obj, "schedule")
	m.Command = strVal(obj, "command")
}

// The cron init script exits 0 even when crond does not start, so a write that
// answered 200 can still leave every job idle. uapi reports what procd sees.
func cronJobWarnings(obj map[string]any, diags *diag.Diagnostics) {
	rt, _ := obj["runtime"].(map[string]any)
	if running := boolVal(rt, "crond_running"); !running.IsNull() && !running.ValueBool() {
		diags.AddWarning("crond is not running",
			"procd reports no running crond, so no cron job runs. The cron init script does not start crond while /etc/crontabs is empty, and stops it when system.cronloglevel is invalid.")
	}
	if asWritten := boolVal(rt, "runs_as_written"); !asWritten.IsNull() && !asWritten.ValueBool() {
		diags.AddWarning("Cron job does not run as written",
			fmt.Sprintf("busybox crond runs the schedule %q differently from how it reads. uapi refuses such a schedule on write, so another writer added this line. Change the schedule to a form uapi accepts, which replaces the job.", strVal(obj, "schedule").ValueString()))
	}
}

func (r *cronJobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cronJobModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"schedule": plan.Schedule.ValueString(), "command": plan.Command.ValueString()}
	obj, _, err := r.client.Post(ctx, "/"+cronJobCollection, body, "")
	if client.IsConflict(err) {
		resp.Diagnostics.AddError("Error creating cron job",
			"The root crontab already holds this schedule and command, from another uapi_cron_job or from outside Terraform. "+
				"To manage the existing line, `terraform import` it by the id in the message below.\n\n"+err.Error())
		return
	}
	if err != nil {
		writeErr(&resp.Diagnostics, "creating", "cron job", err)
		return
	}
	r.read(obj, &plan)
	cronJobWarnings(obj, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cronJobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cronJobModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := r.client.GetObject(ctx, "/"+cronJobCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading cron job", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(obj, &state)
	cronJobWarnings(obj, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cronJobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cronJobModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cronJobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cronJobModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+cronJobCollection+"/"+state.ID.ValueString(), ""); err != nil {
		writeErr(&resp.Diagnostics, "deleting", "cron job", err)
	}
}

func (r *cronJobResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
