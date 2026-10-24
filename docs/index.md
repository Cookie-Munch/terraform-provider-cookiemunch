---
page_title: "Cookie Munch Provider"
description: |-
  Sites, their consent configuration, cookie declarations, webhooks, partner links and connected systems, as code.
---

# Cookie Munch Provider

Manage Cookie Munch with Terraform: the sites your banner runs on, the parts of their configuration you
want in code, cookie declarations, signed webhooks, partner links and the connected systems that carry
out rights requests.

## Example

```terraform
provider "cookiemunch" {} # COOKIEMUNCH_API_KEY

resource "cookiemunch_site" "shop" {
  domain = "shop.example.com"
  name   = "Example Shop"
}

resource "cookiemunch_site_config" "shop" {
  cbid        = cookiemunch_site.shop.cbid
  config_json = jsonencode({ defaultMode = "opt-in", banner = { layout = "bar" } })
}
```

Try it against a sandbox organisation first (`cookiemunch sandbox create` prints a test key): everything
in a sandbox is test data, and erasing it takes everything Terraform made with it.

## Authentication

A Cookie Munch API key, from `api_key` or `COOKIEMUNCH_API_KEY`. Give it only what the configuration
manages: `sites:write` for sites, configuration and cookies, `webhooks:write` for webhooks, `dsar:write`
for connected systems. In CI, a key that lasts an hour from your identity provider
(`cookiemunch token exchange`) leaves no long-lived key in the pipeline.

Every write is strict: a field the API does not know is an error, never silently dropped.

## Schema

### Optional

- `api_key` (String, Sensitive) A Cookie Munch API key. Defaults to `COOKIEMUNCH_API_KEY`.
- `base_url` (String) The API origin. Defaults to `COOKIEMUNCH_BASE_URL`, then `https://api.cookiemunch.net`.
