---
page_title: "cookiemunch_partner_link Resource"
---

# cookiemunch_partner_link

A grant to a partner platform: what it may do for one site or the organisation. No secret is created. A
grant cannot be edited: any change revokes it and grants a new one.

```terraform
resource "cookiemunch_partner_link" "docs" {
  partner  = "forumulate"
  resource = "site:${cookiemunch_site.shop.cbid}"
  scopes   = ["consent:read"]
}
```

## Argument Reference

- `partner` (Required) Any partner the deployment knows (e.g. `forumulate`, `atlas`, `chatly`, `referalli`).
- `resource` (Required) `site:<cbid>` or `org:<org id>`.
- `scopes` (Required) `consent:read`, `receipt:read`, `dsar:read`, `dsar:write`, `dsar:rights-links`, `sites:read`.
- `expires_at`, `external_id`, `label` (Optional).

## Attribute Reference

- `status` `active` or `waiting_on_partner`. A revoked or expired grant is granted again on the next apply.
