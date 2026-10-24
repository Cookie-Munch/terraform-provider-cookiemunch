package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---- cookiemunch_me ----

type meDataSource struct{ c *Client }

type meModel struct {
	OrgID   types.String `tfsdk:"org_id"`
	OrgName types.String `tfsdk:"org_name"`
	Plan    types.String `tfsdk:"plan"`
	Sandbox types.Bool   `tfsdk:"sandbox"`
	Scopes  types.List   `tfsdk:"scopes"`
}

// NewMeDataSource is cookiemunch_me: which organisation the provider's key acts for.
func NewMeDataSource() datasource.DataSource { return &meDataSource{} }

func (d *meDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_me"
}

func (d *meDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The organisation the provider's key acts for, its plan, and whether it is a sandbox.",
		Attributes: map[string]schema.Attribute{
			"org_id":   schema.StringAttribute{Computed: true},
			"org_name": schema.StringAttribute{Computed: true},
			"plan":     schema.StringAttribute{Computed: true, Description: "The effective plan."},
			"sandbox":  schema.BoolAttribute{Computed: true, Description: "True for a sandbox organisation (test keys, test rights requests)."},
			"scopes":   schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Null for a full-access key."},
		},
	}
}

func (d *meDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.c = clientFrom(req.ProviderData, resp.Diagnostics.AddError)
}

func (d *meDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var me struct {
		OrgID   string   `json:"orgId"`
		OrgName *string  `json:"orgName"`
		Plan    string   `json:"plan"`
		Sandbox bool     `json:"sandbox"`
		Scopes  []string `json:"scopes"`
	}
	if err := d.c.Do(ctx, "GET", "/me", nil, &me); err != nil {
		resp.Diagnostics.AddError("Reading /v1/me failed", err.Error())
		return
	}
	m := meModel{OrgID: types.StringValue(me.OrgID), Plan: types.StringValue(me.Plan), Sandbox: types.BoolValue(me.Sandbox), Scopes: listOf(me.Scopes)}
	if me.OrgName != nil {
		m.OrgName = types.StringValue(*me.OrgName)
	} else {
		m.OrgName = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// ---- cookiemunch_site ----

type siteDataSource struct{ c *Client }

type siteDataModel struct {
	Cbid        types.String `tfsdk:"cbid"`
	Domain      types.String `tfsdk:"domain"`
	Name        types.String `tfsdk:"name"`
	Verified    types.Bool   `tfsdk:"verified"`
	VerifyToken types.String `tfsdk:"verify_token"`
}

// NewSiteDataSource is cookiemunch_site (data): a site this configuration does not manage.
func NewSiteDataSource() datasource.DataSource { return &siteDataSource{} }

func (d *siteDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func (d *siteDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A site by cbid.",
		Attributes: map[string]schema.Attribute{
			"cbid":         schema.StringAttribute{Required: true},
			"domain":       schema.StringAttribute{Computed: true},
			"name":         schema.StringAttribute{Computed: true},
			"verified":     schema.BoolAttribute{Computed: true},
			"verify_token": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *siteDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.c = clientFrom(req.ProviderData, resp.Diagnostics.AddError)
}

func (d *siteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m siteDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s apiSite
	if err := d.c.Do(ctx, "GET", "/sites/"+esc(m.Cbid.ValueString()), nil, &s); err != nil {
		resp.Diagnostics.AddError("Reading the site failed", err.Error())
		return
	}
	m.Domain = types.StringValue(s.Domain)
	if s.Name != nil {
		m.Name = types.StringValue(*s.Name)
	} else {
		m.Name = types.StringNull()
	}
	m.Verified = types.BoolValue(s.Verified)
	m.VerifyToken = types.StringValue(s.VerifyToken)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
