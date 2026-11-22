# Terraform / OpenTofu provider for SolidPing

Manage [SolidPing](https://www.solidping.io) monitoring as code: checks, notification channels,
status pages and maintenance windows.

```hcl
terraform {
  required_providers {
    solidping = {
      source = "fclairamb/solidping"
    }
  }
}

provider "solidping" {
  endpoint = "https://solidping.acme.com" # or SOLIDPING_ENDPOINT
  org      = "acme"                       # or SOLIDPING_ORG
  token    = var.solidping_token          # or SOLIDPING_TOKEN
}

resource "solidping_check" "website" {
  name   = "Website"
  type   = "http"
  config = jsonencode({ url = "https://www.acme.com/health" })
}

resource "solidping_status_page" "public" {
  name = "Acme status"
}

resource "solidping_status_page_section" "core" {
  status_page_uid = solidping_status_page.public.id
  name            = "Core services"
  resources       = [{ check_uid = solidping_check.website.id }]
}
```

The token is an organization-scoped personal access token, created in the dashboard or with
`POST /api/v1/orgs/{org}/tokens`.

## What it manages

| Resource | Notes |
|---|---|
| `solidping_check` | Any check type. Type-specific settings in `config` (JSON), secrets in `config_sensitive`. |
| `solidping_channel` | Every channel type except Slack, which is created by installing the Slack app. |
| `solidping_status_page` | The page itself. |
| `solidping_status_page_section` | A section and its components: a hand-curated list, or a selector. |
| `solidping_maintenance_window` | One-off or recurring, with its checks and check groups. |

Data sources `solidping_check`, `solidping_channel` and `solidping_status_page` look up existing
objects by UID, slug or name.

Every resource is imported with `terraform import <address> <org>/<uid>` (a section with
`<org>/<statusPageUid>/<sectionUid>`). Reference documentation: [`docs/`](docs/index.md).

Escalation policies and on-call schedules are not covered yet.

### How secrets are handled

SolidPing stores secrets encrypted and never returns them: a read gives the public settings and
the *names* of the secret keys. The provider therefore splits a JSON object in two attributes:

- `config` / `settings`: public keys. Only the keys you wrote are compared with the server, so
  the defaults and generated values the server adds never show as drift.
- `config_sensitive` / `settings_sensitive`: secret keys, marked sensitive, kept in the state as
  written and never refreshed. Removing a key clears the secret on the server.

## Developing

Requirements: Go (version in `go.mod`), and a SolidPing server to run the acceptance tests against.

```bash
make build     # compile
make lint      # golangci-lint
make test      # unit tests
make docs      # regenerate docs/ from the schemas and examples/
```

Acceptance tests create and delete real objects. Run them against a throwaway server in test
mode, which has a fixed `test@test.com` account:

```bash
docker run --rm -d -p 4000:4000 -e SP_RUNMODE=test ghcr.io/fclairamb/solidping

export SOLIDPING_ENDPOINT=http://localhost:4000 SOLIDPING_ORG=test
export SOLIDPING_ACC_EMAIL=test@test.com SOLIDPING_ACC_PASSWORD=test
make testacc
```

Set `SOLIDPING_TOKEN` instead of the two `SOLIDPING_ACC_*` variables to use an existing token.
The Terraform CLI is downloaded by the test framework when none is installed. To test with
OpenTofu:

```bash
export TF_ACC_TERRAFORM_PATH="$(which tofu)" TF_ACC_PROVIDER_NAMESPACE=hashicorp \
  TF_ACC_PROVIDER_HOST=registry.opentofu.org
```

To try a local build with real configuration, point the CLI at it with a
[dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
after `make install`.

## Releasing

Pushing a `v*` tag runs GoReleaser (`.github/workflows/release.yml`): it builds every platform,
signs `SHA256SUMS` with GPG and publishes a GitHub release in the layout both registries expect.

One-time setup, before the first tag:

1. Generate a GPG key (RSA or DSA, as the registries require) and store it in the password manager.
2. Add the repository secrets `GPG_PRIVATE_KEY` (ASCII-armored private key) and `PASSPHRASE`.
3. Make the repository public.
4. Terraform Registry: sign in on registry.terraform.io with GitHub, add the public key under
   *Settings → Signing keys*, then *Publish → Provider* and pick this repository.
5. OpenTofu Registry: open the "Submit new provider" issue on `opentofu/registry` for
   `fclairamb/terraform-provider-solidping`, and submit the public key with its "Submit new
   provider signing key" issue.

Then, for each release: `git tag v0.1.0 && git push origin v0.1.0`.

## License

[MPL-2.0](LICENSE).
