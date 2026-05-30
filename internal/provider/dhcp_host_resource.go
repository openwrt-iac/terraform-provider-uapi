package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const dhcpHostCollection = "dhcp/hosts"

var (
	_ resource.Resource                = &dhcpHostResource{}
	_ resource.ResourceWithConfigure   = &dhcpHostResource{}
	_ resource.ResourceWithImportState = &dhcpHostResource{}
)

type dhcpHostResource struct {
	client *client.Client
}

func NewDHCPHostResource() resource.Resource {
	return &dhcpHostResource{}
}

type dhcpHostModel struct {
	ID        types.String `tfsdk:"id"`
	Managed   types.Bool   `tfsdk:"managed"`
	Name      types.String `tfsdk:"name"`
	MAC       types.String `tfsdk:"mac"`
	IP        types.String `tfsdk:"ip"`
	Leasetime types.String `tfsdk:"leasetime"`
	Tag       types.String `tfsdk:"tag"`
	DNS       types.Bool   `tfsdk:"dns"`
}

func (r *dhcpHostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_host"
}

func (r *dhcpHostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *dhcpHostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A static DHCP lease (uci dhcp.host).",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Hostname for the static lease.",
			},
			"mac": schema.StringAttribute{
				Required:    true,
				Description: "Client MAC address (aa:bb:cc:dd:ee:ff).",
			},
			"ip": schema.StringAttribute{
				Required:    true,
				Description: "IPv4 or IPv6 address to assign.",
			},
			"leasetime": optionalComputedString("Lease duration like '12h', '30m', '1d', or seconds."),
			"tag":       optionalComputedString("dnsmasq tag to apply to the host."),
			"dns":       optionalComputedBool("Add a DNS entry for the host. Defaults to false."),
		},
	}
}

func (r *dhcpHostResource) body(_ context.Context, m dhcpHostModel) map[string]any {
	out := map[string]any{}
	putStr(out, "name", m.Name)
	putStr(out, "mac", m.MAC)
	putStr(out, "ip", m.IP)
	putStr(out, "leasetime", m.Leasetime)
	putStr(out, "tag", m.Tag)
	putBool(out, "dns", m.DNS)
	return out
}

func (r *dhcpHostResource) read(_ context.Context, obj map[string]any, m *dhcpHostModel) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Name = strVal(obj, "name")
	m.MAC = strVal(obj, "mac")
	m.IP = strVal(obj, "ip")
	m.Leasetime = strVal(obj, "leasetime")
	m.Tag = strVal(obj, "tag")
	m.DNS = boolVal(obj, "dns")
}

func (r *dhcpHostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dhcpHostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Post(ctx, "/"+dhcpHostCollection, r.body(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating dhcp host", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dhcpHostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dhcpHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, "/"+dhcpHostCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading dhcp host", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(ctx, obj, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dhcpHostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dhcpHostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Put(ctx, "/"+dhcpHostCollection+"/"+plan.ID.ValueString(), r.body(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating dhcp host", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dhcpHostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dhcpHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+dhcpHostCollection+"/"+state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting dhcp host", err.Error())
	}
}

func (r *dhcpHostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveImportID(ctx, r.client, dhcpHostCollection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing dhcp host", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
