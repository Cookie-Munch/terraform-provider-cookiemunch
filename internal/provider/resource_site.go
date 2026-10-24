package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// apiSite is the Site object.
type apiSite struct {
	Cbid        string   `json:"cbid"`
	Domain      string   `json:"domain"`
	Name        *string  `json:"name,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Verified    bool     `json:"verified"`
	VerifyToken string   `json:"verifyToken"`
}

type siteModel struct {
	ID          types.String `tfsdk:"id"`
	Cbid        types.String `tfsdk:"cbid"`
	Domain      types.String `tfsdk:"domain"`
	Name        types.String `tfsdk:"name"`
	Aliases     types.List   `tfsdk:"aliases"`
	Platform    types.String `tfsdk:"platform"`
	Verified    types.Bool   `tfsdk:"verified"`
	VerifyToken types.String `tfsdk:"verify_token"`
}

type siteResource struct{ c *Client }

// NewSiteResource is cookiemunch_site.
func NewSiteResource() resource.Resource { return &siteResource{} }

func (r *siteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func (r *siteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A site (property): the domain the banner runs on. Deleting it keeps its consent ledger, as the API always does.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "The cbid.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"cbid": schema.StringAttribute{
				Optional: true, Computed: true,
				Description:   "The site's id in the embed (`data-cbid`). Chosen by the API when omitted. Changing it makes a new site.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"domain":   schema.StringAttribute{Required: true, Description: "A bare hostname. Moving it clears verification."},
			"name":     schema.StringAttribute{Optional: true, Description: "The business name data subjects see on the rights page, in notices and in the policy."},
			"aliases":  schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "Other hosts serving this site (consent-ingest origins and rights-page captcha hosts)."},
			"platform": schema.StringAttribute{Optional: true, Description: "web, ios, android, amp or other. Changing it makes a new site.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"verified": schema.BoolAttribute{Computed: true, Description: "Whether the domain is verified."},
			"verify_token": schema.StringAttribute{
				Computed: true, Description: "The token to publish to verify the domain.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, resp.Diagnostics.AddError)
}

func (r *siteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("cbid"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func stringList(ctx context.Context, l types.List) ([]string, bool) {
	if l.IsNull() || l.IsUnknown() {
		return nil, false
	}
	out := []string{}
	l.ElementsAs(ctx, &out, false)
	return out, true
}

func listOf(v []string) types.List {
	if v == nil {
		return types.ListNull(types.StringType)
	}
	el := make([]attr.Value, 0, len(v))
	for _, s := range v {
		el = append(el, types.StringValue(s))
	}
	return types.ListValueMust(types.StringType, el)
}

// patchBody is what PATCH /v1/sites/{cbid} should send to reach the plan.
func (m siteModel) patchBody(ctx context.Context) map[string]any {
	b := map[string]any{"domain": m.Domain.ValueString()}
	if m.Name.IsNull() {
		b["name"] = nil
	} else {
		b["name"] = m.Name.ValueString()
	}
	if a, ok := stringList(ctx, m.Aliases); ok {
		b["aliases"] = a
	} else {
		b["aliases"] = []string{}
	}
	return b
}

func (m *siteModel) fill(s apiSite) {
	m.ID = types.StringValue(s.Cbid)
	m.Cbid = types.StringValue(s.Cbid)
	m.Domain = types.StringValue(s.Domain)
	if s.Name != nil && *s.Name != "" {
		m.Name = types.StringValue(*s.Name)
	} else {
		m.Name = types.StringNull()
	}
	// An empty alias list in the API and an unset one in the configuration are the same thing.
	if len(s.Aliases) > 0 {
		m.Aliases = listOf(s.Aliases)
	} else if !m.Aliases.IsNull() {
		m.Aliases = listOf([]string{})
	}
	m.Verified = types.BoolValue(s.Verified)
	m.VerifyToken = types.StringValue(s.VerifyToken)
}

func (r *siteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"domain": plan.Domain.ValueString()}
	if !plan.Cbid.IsNull() && !plan.Cbid.IsUnknown() {
		body["cbid"] = plan.Cbid.ValueString()
	}
	if !plan.Platform.IsNull() {
		body["platform"] = plan.Platform.ValueString()
	}
	var created apiSite
	if err := r.c.Do(ctx, "POST", "/sites", body, &created); err != nil {
		resp.Diagnostics.AddError("Creating the site failed", err.Error())
		return
	}
	site := created
	// Name and aliases are set by the same PATCH an update sends.
	if !plan.Name.IsNull() || !plan.Aliases.IsNull() {
		var res struct {
			Site apiSite `json:"site"`
		}
		if err := r.c.Do(ctx, "PATCH", "/sites/"+esc(created.Cbid), plan.patchBody(ctx), &res); err != nil {
			resp.Diagnostics.AddError("Naming the new site failed", err.Error())
			return
		}
		site = res.Site
	}
	plan.fill(site)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *siteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s apiSite
	if err := r.c.Do(ctx, "GET", "/sites/"+esc(state.Cbid.ValueString()), nil, &s); err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Reading the site failed", err.Error())
		return
	}
	state.fill(s)
	if state.Platform.IsUnknown() {
		state.Platform = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *siteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var res struct {
		Site apiSite `json:"site"`
	}
	if err := r.c.Do(ctx, "PATCH", "/sites/"+esc(plan.Cbid.ValueString()), plan.patchBody(ctx), &res); err != nil {
		resp.Diagnostics.AddError("Updating the site failed", err.Error())
		return
	}
	plan.fill(res.Site)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *siteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/sites/"+esc(state.Cbid.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting the site failed", err.Error())
	}
}
