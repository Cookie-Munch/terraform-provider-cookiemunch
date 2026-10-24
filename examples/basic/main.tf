terraform {
  required_providers {
    cookiemunch = {
      source = "cookie-munch/cookiemunch"
    }
  }
}

# The key comes from COOKIEMUNCH_API_KEY. A sandbox key (`cookiemunch sandbox create`) is a good
# place to try this first: everything in a sandbox is test data.
provider "cookiemunch" {}

data "cookiemunch_me" "me" {}

resource "cookiemunch_site" "shop" {
  domain  = "shop.example.com"
  name    = "Example Shop"
  aliases = ["www.shop.example.com"]
}

resource "cookiemunch_site_config" "shop" {
  cbid = cookiemunch_site.shop.cbid
  config_json = jsonencode({
    defaultMode = "opt-in"
    banner      = { layout = "bar" }
  })
}

resource "cookiemunch_cookie_declaration" "session" {
  cbid          = cookiemunch_site.shop.cbid
  name          = "shop_session"
  category      = "necessary"
  purpose       = "Keeps you signed in."
  provider_name = "Example Shop"
  expiry        = "Session"
}

resource "cookiemunch_webhook" "consent" {
  url    = "https://hooks.shop.example.com/cookiemunch"
  events = ["consent.recorded", "dsar.created"]
  cbid   = cookiemunch_site.shop.cbid
}

resource "cookiemunch_partner_link" "docs" {
  partner  = "forumulate"
  resource = "site:${cookiemunch_site.shop.cbid}"
  scopes   = ["consent:read"]
}

output "embed_cbid" {
  value = cookiemunch_site.shop.cbid
}

output "webhook_secret" {
  value     = cookiemunch_webhook.consent.secret
  sensitive = true
}
