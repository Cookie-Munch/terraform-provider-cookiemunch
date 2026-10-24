---
page_title: "cookiemunch_site_config Resource"
---

# cookiemunch_site_config

The fields of a site's configuration this Terraform manages. Only the keys in `config_json` are managed;
the rest keep their saved values or defaults, so the server's defaults never show up as drift. Removing a
key resets it to its default. Destroying the resource leaves the configuration as it is.

```terraform
resource "cookiemunch_site_config" "shop" {
  cbid = cookiemunch_site.shop.cbid
  config_json = jsonencode({
    defaultMode = "opt-in"
    banner      = { layout = "bar" }
    geoRules    = [{ regions = ["eu"], mode = "opt-in" }]
  })
}
```

## Argument Reference

- `cbid` (Required) The site.
- `config_json` (Required) A partial SiteConfig as JSON (every field is in the API's SiteConfig schema). Objects merge; arrays are replaced whole.

## Import

```shell
terraform import cookiemunch_site_config.shop <cbid>
```
An imported configuration manages no keys until the next apply.
