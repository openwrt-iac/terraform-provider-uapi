package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const systemPath = "/system"

var (
	_ resource.Resource                = &systemResource{}
	_ resource.ResourceWithConfigure   = &systemResource{}
	_ resource.ResourceWithImportState = &systemResource{}
)

type systemResource struct {
	client *client.Client
}

func NewSystemResource() resource.Resource {
	return &systemResource{}
}

type systemModel struct {
	ID          types.String `tfsdk:"id"`
	Managed     types.Bool   `tfsdk:"managed"`
	Hostname    types.String `tfsdk:"hostname"`
	Description types.String `tfsdk:"description"`
	Notes       types.String `tfsdk:"notes"`
	Timezone    types.String `tfsdk:"timezone"`
	Zonename    types.String `tfsdk:"zonename"`
	LogSize     types.String `tfsdk:"log_size"`
	LogIP       types.String `tfsdk:"log_ip"`
	LogProto    types.String `tfsdk:"log_proto"`
	LogRemote   types.Bool   `tfsdk:"log_remote"`
	UrandomSeed types.Bool   `tfsdk:"urandom_seed"`
}

func (r *systemResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system"
}

func (r *systemResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *systemResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Global system settings (uci system.system). This is a singleton: it cannot be " +
			"created or destroyed. `terraform destroy` only removes it from state; the underlying " +
			"settings are left as-is on the router.",
		Attributes: map[string]schema.Attribute{
			"id":           computedIDAttribute(),
			"managed":      managedAttribute(),
			"hostname":     optionalComputedString("System hostname."),
			"description":  optionalComputedString("Short device description."),
			"notes":        optionalComputedString("Free-form notes."),
			"timezone":     optionalComputedString("POSIX timezone string (e.g. CET-1CEST,M3.5.0,M10.5.0/3)."),
			"zonename":     optionalComputedString("IANA zone name (e.g. Europe/Paris)."),
			"log_size":     optionalComputedString("Kernel log buffer size in KiB."),
			"log_ip":       optionalComputedString("Remote syslog server IP."),
			"log_proto":    optionalComputedString("Remote syslog protocol (udp or tcp)."),
			"log_remote":   optionalComputedBool("Enable remote logging. Defaults to false."),
			"urandom_seed": optionalComputedBool("Save a random seed across reboots. Defaults to false."),
		},
	}
}

func (r *systemResource) body(_ context.Context, m systemModel) map[string]any {
	out := map[string]any{}
	putStr(out, "hostname", m.Hostname)
	putStr(out, "description", m.Description)
	putStr(out, "notes", m.Notes)
	putStr(out, "timezone", m.Timezone)
	putStr(out, "zonename", m.Zonename)
	putStr(out, "log_size", m.LogSize)
	putStr(out, "log_ip", m.LogIP)
	putStr(out, "log_proto", m.LogProto)
	putBool(out, "log_remote", m.LogRemote)
	putBool(out, "urandom_seed", m.UrandomSeed)
	return out
}

func (r *systemResource) read(_ context.Context, obj map[string]any, m *systemModel) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Hostname = strVal(obj, "hostname")
	m.Description = strVal(obj, "description")
	m.Notes = strVal(obj, "notes")
	m.Timezone = strVal(obj, "timezone")
	m.Zonename = strVal(obj, "zonename")
	m.LogSize = strVal(obj, "log_size")
	m.LogIP = strVal(obj, "log_ip")
	m.LogProto = strVal(obj, "log_proto")
	m.LogRemote = boolVal(obj, "log_remote")
	m.UrandomSeed = boolVal(obj, "urandom_seed")
}

func (r *systemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan systemModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Patch(ctx, systemPath, r.body(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error configuring system settings", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *systemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state systemModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, systemPath)
	if err != nil {
		resp.Diagnostics.AddError("Error reading system settings", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(ctx, obj, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *systemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan systemModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Patch(ctx, systemPath, r.body(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating system settings", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the system singleton cannot be removed. State is dropped
// by the framework once this returns.
func (r *systemResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *systemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
