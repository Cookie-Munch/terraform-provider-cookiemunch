---
page_title: "cookiemunch_webhook Resource"
---

# cookiemunch_webhook

A signed webhook subscription.

```terraform
resource "cookiemunch_webhook" "consent" {
  url    = "https://hooks.shop.example.com/cookiemunch"
  events = ["consent.recorded", "dsar.created"]
}
```

## Argument Reference

- `url` (Required) An https URL.
- `events` (Required) e.g. `consent.recorded`, `dsar.created`, `scan.completed`.
- `cbid` (Optional) Limit it to one site.
- `active` (Optional) `false` pauses delivery. Default `true`.

## Attribute Reference

- `secret` (Sensitive) The signing secret, known only from creation: an imported webhook has none.
- `state` `healthy`, `failing`, `disabled` or `paused`.

## Import

```shell
terraform import cookiemunch_webhook.consent <id>
```
