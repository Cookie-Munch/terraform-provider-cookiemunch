package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeAPI is an in-memory Cookie Munch Developer API with just the routes the provider uses. It
// refuses a write without the strict and version headers, so the provider must always send them.
type fakeAPI struct {
	mu        sync.Mutex
	n         int
	sites     map[string]map[string]any
	configs   map[string]map[string]any
	cookies   map[string]map[string]map[string]any
	webhooks  map[string]map[string]any
	links     map[string]map[string]any
	executors map[string]map[string]any
}

func newFake() *fakeAPI {
	return &fakeAPI{
		sites: map[string]map[string]any{}, configs: map[string]map[string]any{}, cookies: map[string]map[string]map[string]any{},
		webhooks: map[string]map[string]any{}, links: map[string]map[string]any{}, executors: map[string]map[string]any{},
	}
}

func (f *fakeAPI) id(p string) string { f.n++; return fmt.Sprintf("%s_%d", p, f.n) }

// merge applies an RFC 7386 merge patch.
func merge(base map[string]any, patch map[string]any) map[string]any {
	for k, v := range patch {
		if v == nil {
			delete(base, k)
			continue
		}
		if pm, ok := v.(map[string]any); ok {
			bm, _ := base[k].(map[string]any)
			if bm == nil {
				bm = map[string]any{}
			}
			base[k] = merge(bm, pm)
			continue
		}
		base[k] = v
	}
	return base
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	send := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if v != nil {
			_ = json.NewEncoder(w).Encode(v)
		}
	}
	if r.Header.Get("Authorization") != "Bearer cookiemunch_test_org_k" || r.Header.Get("CookieMunch-Version") != APIVersion {
		send(401, map[string]any{"error": "bad key or version"})
		return
	}
	var body map[string]any
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.Header.Get("Prefer") != "strict-unknown-fields" {
			send(400, map[string]any{"error": "the provider must write strictly"})
			return
		}
	}
	seg := strings.Split(strings.TrimPrefix(r.URL.EscapedPath(), "/v1/"), "/")
	route := r.Method + " " + seg[0]
	switch {
	case route == "GET me":
		send(200, map[string]any{"orgId": "org_1", "orgName": "Acme", "plan": "pro", "sandbox": true, "scopes": nil})
	case route == "POST sites":
		cbid, _ := body["cbid"].(string)
		if cbid == "" {
			cbid = f.id("site")
		}
		s := map[string]any{"cbid": cbid, "orgId": "org_1", "domain": body["domain"], "verified": false, "verifyToken": "tok_" + cbid}
		f.sites[cbid] = s
		send(201, s)
	case seg[0] == "sites" && len(seg) == 2:
		s := f.sites[seg[1]]
		if s == nil {
			send(404, map[string]any{"error": "site not found"})
			return
		}
		switch r.Method {
		case "GET":
			send(200, s)
		case "PATCH":
			for k, v := range body {
				if v == nil || (k == "aliases" && len(v.([]any)) == 0) {
					delete(s, k)
				} else {
					s[k] = v
				}
			}
			send(200, map[string]any{"site": s, "diff": []any{}})
		case "DELETE":
			delete(f.sites, seg[1])
			send(204, nil)
		}
	case seg[0] == "sites" && len(seg) == 3 && seg[2] == "config":
		c := f.configs[seg[1]]
		if c == nil {
			// The effective config: defaults included.
			c = map[string]any{"defaultMode": "opt-in", "banner": map[string]any{"layout": "box", "position": "bottom"}}
			f.configs[seg[1]] = c
		}
		if r.Method == "PATCH" {
			merge(c, body)
		}
		send(200, c)
	case seg[0] == "sites" && len(seg) == 3 && seg[2] == "cookies" && r.Method == "GET":
		declared := []any{}
		for _, d := range f.cookies[seg[1]] {
			declared = append(declared, d)
		}
		send(200, map[string]any{"updatedAt": 1, "cookies": []any{}, "declared": declared})
	case seg[0] == "sites" && len(seg) == 4 && seg[2] == "cookies":
		if f.cookies[seg[1]] == nil {
			f.cookies[seg[1]] = map[string]map[string]any{}
		}
		if r.Method == "PUT" {
			body["name"] = seg[3]
			f.cookies[seg[1]][seg[3]] = body
			send(200, map[string]any{"declared": body})
		} else {
			delete(f.cookies[seg[1]], seg[3])
			send(204, nil)
		}
	case route == "POST webhooks":
		id := f.id("wh")
		wh := map[string]any{"id": id, "url": body["url"], "events": body["events"], "cbid": body["cbid"], "active": true, "state": "healthy"}
		f.webhooks[id] = wh
		out := map[string]any{"secret": "whsec_" + id}
		for k, v := range wh {
			out[k] = v
		}
		send(201, out)
	case route == "GET webhooks":
		all := []any{}
		for _, wh := range f.webhooks {
			all = append(all, wh)
		}
		send(200, all)
	case seg[0] == "webhooks" && len(seg) == 2:
		wh := f.webhooks[seg[1]]
		if wh == nil {
			send(404, map[string]any{"error": "unknown"})
			return
		}
		if r.Method == "PATCH" {
			for k, v := range body {
				wh[k] = v
			}
			if wh["active"] == false {
				wh["state"] = "paused"
			} else {
				wh["state"] = "healthy"
			}
			send(200, wh)
		} else {
			delete(f.webhooks, seg[1])
			send(204, nil)
		}
	case route == "POST partner_links":
		id := f.id("pl")
		l := map[string]any{"id": id, "partner": body["partner"], "resource": body["resource"], "scopes": body["scopes"], "status": "active",
			"created_at": "2026-10-24T00:00:00Z", "expires_at": nil, "last_used_at": nil, "calls_30d": 0, "externalId": body["resource"]}
		if body["label"] != nil {
			l["label"] = body["label"]
		}
		f.links[id] = l
		send(201, l)
	case route == "GET partner_links":
		all := []any{}
		for _, l := range f.links {
			all = append(all, l)
		}
		send(200, all)
	case route == "DELETE partner_links":
		l := f.links[seg[1]]
		if l == nil {
			send(404, map[string]any{"error": "no such link"})
			return
		}
		l["status"] = "revoked"
		send(200, l)
	case route == "POST dsar":
		id := f.id("ex")
		system, _ := body["system"].(string)
		if system == "" {
			system, _ = body["preset"].(string)
		}
		e := map[string]any{"id": id, "system": system, "baseUrl": body["baseUrl"], "keyPrefix": "sk_…" + id[len(id)-1:], "auto": true}
		f.executors[id] = e
		out := map[string]any{"webhookUrl": "https://api.test/hooks/" + id}
		for k, v := range e {
			out[k] = v
		}
		send(201, out)
	case seg[0] == "dsar" && len(seg) == 3 && seg[1] == "executors":
		e := f.executors[seg[2]]
		if e == nil {
			send(404, map[string]any{"error": "no such connection"})
			return
		}
		if r.Method == "DELETE" {
			delete(f.executors, seg[2])
			send(204, nil)
			return
		}
		send(200, e)
	default:
		send(404, map[string]any{"error": "no route " + route})
	}
}

func factories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"cookiemunch": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func providerBlock(url string) string {
	return fmt.Sprintf("provider \"cookiemunch\" {\n  api_key  = \"cookiemunch_test_org_k\"\n  base_url = %q\n}\n", url)
}

func TestEverythingAsCode(t *testing.T) {
	fake := newFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	p := providerBlock(srv.URL)

	step1 := p + `
data "cookiemunch_me" "me" {}

resource "cookiemunch_site" "shop" {
  cbid    = "shop"
  domain  = "shop.example"
  name    = "Acme Shop"
  aliases = ["www.shop.example"]
}

resource "cookiemunch_site_config" "shop" {
  cbid        = cookiemunch_site.shop.cbid
  config_json = jsonencode({ defaultMode = "opt-out", banner = { layout = "bar" } })
}

resource "cookiemunch_cookie_declaration" "session" {
  cbid          = cookiemunch_site.shop.cbid
  name          = "shop_session"
  category      = "necessary"
  purpose       = "Keeps you signed in."
  provider_name = "Acme"
  expiry        = "Session"
}

resource "cookiemunch_webhook" "events" {
  url    = "https://hooks.example/cm"
  events = ["consent.recorded", "dsar.created"]
  cbid   = cookiemunch_site.shop.cbid
}

resource "cookiemunch_partner_link" "docs" {
  partner  = "forumulate"
  resource = "site:${cookiemunch_site.shop.cbid}"
  scopes   = ["consent:read"]
  label    = "docs"
}

resource "cookiemunch_dsar_connection" "crm" {
  system     = "crm"
  base_url   = "https://crm.example"
  secret_key = "sk_live_secret"
}
`
	// The same, with the config narrowed, the cookie re-described and the webhook paused.
	step2 := strings.Replace(strings.Replace(strings.Replace(step1,
		`jsonencode({ defaultMode = "opt-out", banner = { layout = "bar" } })`, `jsonencode({ banner = { layout = "bar" } })`, 1),
		`purpose       = "Keeps you signed in."`, `purpose       = "Keeps your basket."`, 1),
		`cbid   = cookiemunch_site.shop.cbid
}`, `cbid   = cookiemunch_site.shop.cbid
  active = false
}`, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories(),
		Steps: []resource.TestStep{
			{
				Config: step1,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cookiemunch_me.me", "org_id", "org_1"),
					resource.TestCheckResourceAttr("data.cookiemunch_me.me", "sandbox", "true"),
					resource.TestCheckResourceAttr("cookiemunch_site.shop", "verify_token", "tok_shop"),
					resource.TestCheckResourceAttr("cookiemunch_site.shop", "aliases.0", "www.shop.example"),
					// Terraform creates in parallel, so the fake's ids come in any order.
					resource.TestMatchResourceAttr("cookiemunch_webhook.events", "secret", regexp.MustCompile(`^whsec_wh_\d+$`)),
					resource.TestCheckResourceAttr("cookiemunch_partner_link.docs", "status", "active"),
					resource.TestCheckResourceAttr("cookiemunch_partner_link.docs", "external_id", "site:shop"),
					resource.TestMatchResourceAttr("cookiemunch_dsar_connection.crm", "webhook_url", regexp.MustCompile(`^https://api\.test/hooks/ex_\d+$`)),
					func(*terraform.State) error {
						fake.mu.Lock()
						defer fake.mu.Unlock()
						want := map[string]any{"defaultMode": "opt-out", "banner": map[string]any{"layout": "bar", "position": "bottom"}}
						if !reflect.DeepEqual(fake.configs["shop"], want) {
							return fmt.Errorf("config is %v", fake.configs["shop"])
						}
						if fake.cookies["shop"]["shop_session"]["provider"] != "Acme" {
							return fmt.Errorf("declaration is %v", fake.cookies["shop"]["shop_session"])
						}
						return nil
					},
				),
			},
			{
				// No change: a re-plan of the same configuration is empty (no drift from server defaults).
				Config:   step1,
				PlanOnly: true,
			},
			{
				Config: step2,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cookiemunch_webhook.events", "state", "paused"),
					// the secret is kept from creation
					resource.TestMatchResourceAttr("cookiemunch_webhook.events", "secret", regexp.MustCompile(`^whsec_wh_\d+$`)),
					func(*terraform.State) error {
						fake.mu.Lock()
						defer fake.mu.Unlock()
						// defaultMode left the configuration, so it went back to its default (removed from the stored config).
						if _, still := fake.configs["shop"]["defaultMode"]; still {
							return fmt.Errorf("defaultMode should have been reset: %v", fake.configs["shop"])
						}
						if fake.cookies["shop"]["shop_session"]["purpose"] != "Keeps your basket." {
							return fmt.Errorf("purpose not updated")
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "cookiemunch_site.shop",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"platform"},
			},
			{
				ResourceName:      "cookiemunch_cookie_declaration.session",
				ImportState:       true,
				ImportStateId:     "shop/shop_session",
				ImportStateVerify: true,
			},
		},
		CheckDestroy: func(*terraform.State) error {
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if len(fake.sites) != 0 || len(fake.webhooks) != 0 || len(fake.executors) != 0 || len(fake.cookies["shop"]) != 0 {
				return fmt.Errorf("left behind: sites %d, webhooks %d, executors %d", len(fake.sites), len(fake.webhooks), len(fake.executors))
			}
			for _, l := range fake.links {
				if l["status"] != "revoked" {
					return fmt.Errorf("partner link %v not revoked", l["id"])
				}
			}
			return nil
		},
	})
}

func TestProjectAndRemovals(t *testing.T) {
	live := map[string]any{"a": 1.0, "b": map[string]any{"c": 2.0, "d": 3.0}, "e": []any{1.0}}
	if got := project(live, map[string]any{"b": map[string]any{"c": nil}}); !reflect.DeepEqual(got, map[string]any{"b": map[string]any{"c": 2.0}}) {
		t.Fatalf("project: %v", got)
	}
	old := map[string]any{"a": 1.0, "b": map[string]any{"c": 2.0, "d": 3.0}}
	nw := map[string]any{"b": map[string]any{"c": 5.0}}
	if got := removals(old, nw); !reflect.DeepEqual(got, map[string]any{"a": nil, "b": map[string]any{"d": nil}}) {
		t.Fatalf("removals: %v", got)
	}
	if got := mergeInto(map[string]any{"b": map[string]any{"c": 5.0}}, removals(old, nw)); !reflect.DeepEqual(got, map[string]any{"a": nil, "b": map[string]any{"c": 5.0, "d": nil}}) {
		t.Fatalf("mergeInto: %v", got)
	}
}
