---
page_title: "cookiemunch_site Resource"
---

# cookiemunch_site

A site (property): the domain the banner runs on. Deleting it keeps its consent ledger, as the API always does.

```terraform
resource "cookiemunch_site" "shop" {
  domain  = "shop.example.com"
  name    = "Example Shop"
  aliases = ["www.shop.example.com"]
}
```

## Argument Reference

- `domain` (Required) A bare hostname. Moving it clears verification.
- `cbid` (Optional) The id in the embed (`data-cbid`). Chosen by the API when omitted. Changing it makes a new site.
- `name` (Optional) The business name data subjects see on the rights page, in notices and in the policy.
- `aliases` (Optional) Other hosts serving the site.
- `platform` (Optional) `web`, `ios`, `android`, `amp` or `other`. Changing it makes a new site.

## Attribute Reference

- `id`, `verified`, `verify_token` (publish it to verify the domain).

## Import

```shell
terraform import cookiemunch_site.shop <cbid>
```
