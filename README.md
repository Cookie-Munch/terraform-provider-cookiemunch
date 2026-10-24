# Terraform provider for Cookie Munch

Sites, their consent configuration, cookie declarations, webhooks, partner links and connected systems, as code.

```terraform
terraform {
  required_providers {
    cookiemunch = { source = "cookie-munch/cookiemunch" }
  }
}

provider "cookiemunch" {} # COOKIEMUNCH_API_KEY

resource "cookiemunch_site" "shop" {
  domain = "shop.example.com"
}
```

| Resource | What it manages |
|---|---|
| `cookiemunch_site` | A site: domain, display name, aliases |
| `cookiemunch_site_config` | The configuration keys you name (the rest keep their defaults) |
| `cookiemunch_cookie_declaration` | What one cookie is |
| `cookiemunch_webhook` | A signed webhook subscription |
| `cookiemunch_partner_link` | A grant to a partner platform |
| `cookiemunch_dsar_connection` | A connected system that runs part of a rights request |

Data sources: `cookiemunch_me`, `cookiemunch_site`. A full example is in [`examples/basic`](examples/basic/main.tf);
every resource is documented in [`docs/`](docs/).

Try it against a sandbox organisation first: `cookiemunch sandbox create` prints a test key, and erasing
the sandbox takes everything Terraform made in it.

## Developing

```shell
go vet ./...
go test ./...   # drives the provider through real Terraform runs against an in-memory API
```

The tests need `terraform` on the PATH or will fetch it. Releases are signed and built by GoReleaser
when a `v*` tag is pushed.
