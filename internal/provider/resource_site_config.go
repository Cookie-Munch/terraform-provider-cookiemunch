package provider

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type siteConfigModel struct {
	ID     types.String `tfsdk:"id"`
	Cbid   types.String `tfsdk:"cbid"`
	Config types.String `tfsdk:"config_json"`
}

type siteConfigResource struct{ c *Client }

// NewSiteConfigResource is cookiemunch_site_config.
func NewSiteConfigResource() resource.Resource { return &siteConfigResource{} }

func (r *siteConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_config"
}

func (r *siteConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The fields of a site's configuration this Terraform manages. Only the keys in `config_json` are managed: " +
			"the rest keep their saved values or defaults. Removing a key resets it to its default. Destroying this resource " +
			"leaves the configuration as it is.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"cbid": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"config_json": schema.StringAttribute{
				Required:    true,
				Description: "A partial SiteConfig as JSON, e.g. `jsonencode({ defaultMode = \"opt-in\", banner = { layout = \"bar\" } })`. Every field is in the SiteConfig schema of the API.",
			},
		},
	}
}

func (r *siteConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, resp.Diagnostics.AddError)
}

func (r *siteConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Imported with no managed keys: the next plan shows every key the configuration names as a change.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cbid"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("config_json"), "{}")...)
}

// project keeps, from actual, exactly the keys shape names (recursively through objects).
// Arrays and scalars are taken whole: a merge patch replaces an array, it never merges it.
func project(actual, shape any) any {
	sm, ok := shape.(map[string]any)
	if !ok {
		return actual
	}
	am, ok := actual.(map[string]any)
	if !ok {
		return actual
	}
	out := map[string]any{}
	for k, v := range sm {
		if av, present := am[k]; present {
			out[k] = project(av, v)
		}
	}
	return out
}

// removals is the merge patch that drops the keys old had and new does not: `null` resets a field.
func removals(old, new any) map[string]any {
	om, ok := old.(map[string]any)
	if !ok {
		return nil
	}
	nm, _ := new.(map[string]any)
	out := map[string]any{}
	for k, ov := range om {
		nv, kept := nm[k]
		if !kept {
			out[k] = nil
			continue
		}
		if sub := removals(ov, nv); len(sub) > 0 {
			out[k] = sub
		}
	}
	return out
}

// mergeInto lays patch over base (both merge patches), for one request.
func mergeInto(base map[string]any, patch map[string]any) map[string]any {
	for k, v := range patch {
		if pm, ok := v.(map[string]any); ok {
			if bm, ok := base[k].(map[string]any); ok {
				base[k] = mergeInto(bm, pm)
				continue
			}
			if _, set := base[k]; set {
				continue // the new value wins over a removal under it
			}
		}
		if _, set := base[k]; !set {
			base[k] = v
		}
	}
	return base
}

func decode(s string) (any, error) {
	var v any
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}

func (r *siteConfigResource) apply(ctx context.Context, cbid, oldJSON, newJSON string) error {
	nv, err := decode(newJSON)
	if err != nil {
		return err
	}
	patch, _ := nv.(map[string]any)
	if patch == nil {
		patch = map[string]any{}
	}
	if oldJSON != "" {
		if ov, err := decode(oldJSON); err == nil {
			patch = mergeInto(patch, removals(ov, nv))
		}
	}
	return r.c.Do(ctx, "PATCH", "/sites/"+esc(cbid)+"/config", patch, nil)
}

func (r *siteConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, plan.Cbid.ValueString(), "", plan.Config.ValueString()); err != nil {
		resp.Diagnostics.AddError("Writing the site configuration failed", err.Error())
		return
	}
	plan.ID = plan.Cbid
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *siteConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var live map[string]any
	if err := r.c.Do(ctx, "GET", "/sites/"+esc(state.Cbid.ValueString())+"/config", nil, &live); err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Reading the site configuration failed", err.Error())
		return
	}
	shape, err := decode(state.Config.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("The stored config_json is not JSON", err.Error())
		return
	}
	seen := project(live, shape)
	// Keep the configuration's own spelling when nothing changed; show what is live when it did.
	if !reflect.DeepEqual(seen, shape) {
		b, _ := json.Marshal(seen)
		state.Config = types.StringValue(string(b))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *siteConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state siteConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, plan.Cbid.ValueString(), state.Config.ValueString(), plan.Config.ValueString()); err != nil {
		resp.Diagnostics.AddError("Writing the site configuration failed", err.Error())
		return
	}
	plan.ID = plan.Cbid
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete leaves the configuration as it is: the site's banner keeps working.
func (r *siteConfigResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
