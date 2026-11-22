# Changelog

## 0.1.0 (2026-11-22)


### Features

First release of the Terraform / OpenTofu provider for SolidPing.

* Provider configuration `endpoint` / `org` / `token`, with the `SOLIDPING_ENDPOINT`, `SOLIDPING_ORG` and `SOLIDPING_TOKEN` environment fallbacks
* Resources: `solidping_check`, `solidping_channel` (every type except Slack), `solidping_status_page`, `solidping_status_page_section`, `solidping_maintenance_window`
* Data sources: `solidping_check`, `solidping_channel`, `solidping_status_page`
* `terraform import` with `org/uid` on every resource
* Secrets in `config_sensitive` / `settings_sensitive`, stored encrypted by SolidPing and never read back
