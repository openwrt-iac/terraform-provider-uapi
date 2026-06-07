package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &authorizedKeyDataSource{}
	_ datasource.DataSourceWithConfigure = &authorizedKeyDataSource{}
)

type authorizedKeyDataSource struct{ client *client.Client }

func NewAuthorizedKeyDataSource() datasource.DataSource { return &authorizedKeyDataSource{} }

type authorizedKeyDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Type    types.String `tfsdk:"type"`
	Comment types.String `tfsdk:"comment"`
}

func (d *authorizedKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_authorized_key"
}

func (d *authorizedKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *authorizedKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up an SSH authorized key by id. The key blob itself is never returned by uapi.",
		Attributes: map[string]dsschema.Attribute{
			"id":      dsIDAttribute(),
			"type":    dsComputedString("SSH key type (e.g. ssh-ed25519, ssh-rsa, ecdsa-sha2-nistp256)."),
			"comment": dsComputedString("Optional comment (trailing text on the key line)."),
		},
	}
}

func (d *authorizedKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m authorizedKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := d.client.GetObject(ctx, "/"+authorizedKeyCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading authorized key", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Authorized key not found", "No authorized key with id "+m.ID.ValueString())
		return
	}
	m.ID = strVal(obj, "id")
	m.Type = strVal(obj, "type")
	m.Comment = strVal(obj, "comment")
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
