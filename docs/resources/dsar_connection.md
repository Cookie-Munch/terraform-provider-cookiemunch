---
page_title: "cookiemunch_dsar_connection Resource"
---

# cookiemunch_dsar_connection

A connected system that runs part of a rights request, from a preset (`GET /v1/dsar/executors/presets`)
or a profile. Credentials are sent once and never read back. Any change makes a new connection.

```terraform
resource "cookiemunch_dsar_connection" "crm" {
  system     = "crm"
  base_url   = "https://crm.shop.example.com"
  secret_key = var.crm_api_key
}
```

## Argument Reference

- `secret_key` (Required, Sensitive) The system's API key. Stored encrypted; never returned.
- `preset`, `preset_version`, `preset_org_id` (Optional) Build the profile from a preset.
- `profile_json` (Optional) An ExecutorProfile as JSON, instead of a preset.
- `system`, `base_url`, `webhook_secret`, `cbid`, `auto`, `subject_note`, `label` (Optional).

## Attribute Reference

- `webhook_url` Point the system's webhook endpoint here. `key_prefix` names the stored key.
