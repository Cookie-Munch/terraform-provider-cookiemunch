---
page_title: "cookiemunch_cookie_declaration Resource"
---

# cookiemunch_cookie_declaration

What one cookie (or `prefix*` wildcard, or browser-storage key) is. Overrides scanned and observed classification wherever the declaration is shown.

```terraform
resource "cookiemunch_cookie_declaration" "session" {
  cbid          = cookiemunch_site.shop.cbid
  name          = "shop_session"
  category      = "necessary"
  purpose       = "Keeps you signed in."
  provider_name = "Example Shop"
  expiry        = "Session"
}
```

## Argument Reference

- `cbid`, `name` (Required) The site and the exact name, or `prefix*`. Changing either makes a new declaration.
- `category` (Required) `necessary`, `preferences`, `statistics`, `marketing`, or a custom category id.
- `purpose` (Required) Shown to visitors.
- `provider_name`, `expiry`, `domain` (Optional).
- `type` (Optional) `cookie` (the default), `localStorage`, `sessionStorage` or `indexedDB`.

## Import

```shell
terraform import cookiemunch_cookie_declaration.session <cbid>/<name>
```
