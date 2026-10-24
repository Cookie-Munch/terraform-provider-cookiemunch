package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type apiDeclared struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Purpose  string `json:"purpose"`
	Provider string `json:"provider,omitempty"`
	Expiry   string `json:"expiry,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Type     string `json:"type,omitempty"`
}

type cookieModel struct {
	ID       types.String `tfsdk:"id"`
	Cbid     types.String `tfsdk:"cbid"`
	Name     types.String `tfsdk:"name"`
	Category types.String `tfsdk:"category"`
	Purpose  types.String `tfsdk:"purpose"`
	Provider types.String `tfsdk:"provider_name"`
	Expiry   types.String `tfsdk:"expiry"`
	Domain   types.String `tfsdk:"domain"`
	Type     types.String `tfsdk:"type"`
}

type cookieResource struct{ c *Client }

// NewCookieDeclarationResource is cookiemunch_cookie_declaration.
func NewCookieDeclarationResource() resource.Resource { return &cookieResource{} }

func (r *cookieResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cookie_declaration"
}

func (r *cookieResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	opt := func(d string) schema.StringAttribute { return schema.StringAttribute{Optional: true, Description: d} }
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "What one cookie (or `prefix*` wildcard, or browser-storage key) is. Overrides scanned and observed classification everywhere the declaration is shown.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "`<cbid>/<name>`.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"cbid":          schema.StringAttribute{Required: true, PlanModifiers: replace},
			"name":          schema.StringAttribute{Required: true, Description: "The exact name, or `prefix*`.", PlanModifiers: replace},
			"category":      schema.StringAttribute{Required: true, Description: "necessary, preferences, statistics, marketing, or one of the site's custom category ids."},
			"purpose":       schema.StringAttribute{Required: true, Description: "Shown to visitors."},
			"provider_name": opt("Who sets it."),
			"expiry":        opt("Free text, e.g. `Session`, `30 days`."),
			"domain":        opt("The domain it is set on."),
			"type":          opt("cookie (the default), localStorage, sessionStorage or indexedDB."),
		},
	}
}

func (r *cookieResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, resp.Diagnostics.AddError)
}

func (r *cookieResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	cbid, name, ok := strings.Cut(req.ID, "/")
	if !ok || cbid == "" || name == "" {
		resp.Diagnostics.AddError("Import id", "Use `<cbid>/<cookie name>`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cbid"), cbid)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

func optStr(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

func strOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (r *cookieResource) put(ctx context.Context, m *cookieModel) error {
	body := map[string]any{"category": m.Category.ValueString(), "purpose": m.Purpose.ValueString()}
	for k, v := range map[string]types.String{"provider": m.Provider, "expiry": m.Expiry, "domain": m.Domain, "type": m.Type} {
		if s := optStr(v); s != "" {
			body[k] = s
		}
	}
	if err := r.c.Do(ctx, "PUT", "/sites/"+esc(m.Cbid.ValueString())+"/cookies/"+esc(m.Name.ValueString()), body, nil); err != nil {
		return err
	}
	m.ID = types.StringValue(fmt.Sprintf("%s/%s", m.Cbid.ValueString(), m.Name.ValueString()))
	return nil
}

func (r *cookieResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cookieModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Declaring the cookie failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *cookieResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cookieModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var list struct {
		Declared []apiDeclared `json:"declared"`
	}
	if err := r.c.Do(ctx, "GET", "/sites/"+esc(state.Cbid.ValueString())+"/cookies", nil, &list); err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Reading the cookie declarations failed", err.Error())
		return
	}
	for _, d := range list.Declared {
		if d.Name != state.Name.ValueString() {
			continue
		}
		state.Category = types.StringValue(d.Category)
		state.Purpose = types.StringValue(d.Purpose)
		state.Provider = strOrNull(d.Provider)
		state.Expiry = strOrNull(d.Expiry)
		state.Domain = strOrNull(d.Domain)
		// A plain cookie may come back with or without `type`; keep what the configuration says.
		if d.Type != "" && !(d.Type == "cookie" && state.Type.IsNull()) {
			state.Type = types.StringValue(d.Type)
		}
		state.ID = types.StringValue(fmt.Sprintf("%s/%s", state.Cbid.ValueString(), d.Name))
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *cookieResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cookieModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Declaring the cookie failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *cookieResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cookieModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/sites/"+esc(state.Cbid.ValueString())+"/cookies/"+esc(state.Name.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Removing the declaration failed", err.Error())
	}
}
