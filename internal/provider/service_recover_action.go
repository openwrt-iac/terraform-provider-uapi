package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ action.Action              = &serviceRecoverAction{}
	_ action.ActionWithConfigure = &serviceRecoverAction{}
)

type serviceRecoverAction struct{ client *client.Client }

func NewServiceRecoverAction() action.Action { return &serviceRecoverAction{} }

type serviceRecoverModel struct {
	Service types.String `tfsdk:"service"`
}

func (a *serviceRecoverAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_recover"
}

func (a *serviceRecoverAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	if c, ok := req.ProviderData.(*client.Client); ok {
		a.client = c
	}
}

func (a *serviceRecoverAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actschema.Schema{
		Description: "Starts a service again when procd gave up on one of its instances, through " +
			"`POST /uapi/services/{name}/recover`. procd gives up on an instance that dies too often soon after a start: " +
			"a write then warns that a daemon is down, and `data.uapi_diagnostics` lists the instance under " +
			"`stopped_instances`. uapi stops and starts the whole service once, so the instances of the service that " +
			"still run restart too. It starts nothing when procd gave up on no instance. Run it with " +
			"`terraform apply -invoke=action.uapi_service_recover.<name>`, or from the `action_trigger` of a resource. " +
			"Needs Terraform 1.14 or later. The token needs `uapi:services:rw`, or `rw` on a domain whose writes reload " +
			"the service, such as `dhcp:rw` for dnsmasq.",
		Attributes: map[string]actschema.Attribute{
			"service": actschema.StringAttribute{
				Required: true,
				Description: "Service to start again: `dnsmasq`, `lldpd`, `mwan3`, `odhcpd`, " +
					"`prometheus-node-exporter-lua`, `snmpd`, `sysntpd`, `unbound` or `vnstat`. uapi answers 404 for " +
					"another name, `network`, `uhttpd` and `dropbear` included: a stop of one of them cuts off the request " +
					"or the management SSH session.",
			},
		},
	}
}

func (a *serviceRecoverAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var cfg serviceRecoverModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := cfg.Service.ValueString()
	obj, _, err := a.client.Post(ctx, "/uapi/services/"+url.PathEscape(name)+"/recover", nil, "")
	if err != nil {
		resp.Diagnostics.AddError("Error recovering "+name, err.Error())
		return
	}
	var names []string
	if arr, ok := obj["recovered"].([]any); ok {
		for _, v := range arr {
			names = append(names, fmt.Sprint(v))
		}
	}
	msg := "procd had given up on no instance of " + name + ", so uapi started nothing."
	if len(names) > 0 {
		msg = "uapi stopped and started " + name + ". These instances run again: " + strings.Join(names, ", ") + "."
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: msg})
	}
}
