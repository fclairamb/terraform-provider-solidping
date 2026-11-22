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

Releases are cut by [release-please](https://github.com/googleapis/release-please)
(`.github/workflows/release-please.yml`). It keeps a `chore(release): vX.Y.Z` pull request up to
date with the changelog of what was merged since the last release. Merging that pull request tags
the version and creates the GitHub release; GoReleaser then builds every platform, signs
`SHA256SUMS` with GPG and attaches the artifacts in the layout both registries read.

Pull requests are squash-merged with their title as the commit subject, so the title must be a
conventional commit (`feat: ...`, `fix(check): ...`): `feat` and `fix` are what make a release.

Secrets and where they live (gopass paths, never the values):

| Repository secret | gopass | Purpose |
|---|---|---|
| `GPG_PRIVATE_KEY`, `PASSPHRASE` | `personal/solidping/terraform-provider/gpg-private-key`, `gpg-passphrase` | Sign `SHA256SUMS`. The public half (`gpg-public-key`) is registered on registry.terraform.io and in the OpenTofu registry. |
| `CI_PAT` | `github/terraform-provider-solidping/ci-pat` | Fine-grained token (contents and pull requests, read and write, this repository) release-please and the auto-merge workflow act with, so that what they push triggers CI. Without it they fall back to `GITHUB_TOKEN`: start the Tests workflow by hand on the release PR. |

## Automation

- A pull request labelled `automerge` merges on its own once CI is green
  (`.github/workflows/auto-merge.yml` enables auto-merge and keeps the branch up to date).
- Renovate groups non-major updates and auto-merges them (`.github/renovate.json`).
- The `autopilot`, `will-automerge` and `autorelease` labels and `specs/todos/` are the same
  conventions as in the SolidPing repository.

## License

[MPL-2.0](LICENSE).
