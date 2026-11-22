# AGENTS.md

Agent instructions for every coding agent. `CLAUDE.md` is a one-line `@AGENTS.md` stub: edit this file.

Terraform / OpenTofu provider for SolidPing (`../solidping`), built with the Terraform Plugin
Framework. Spec and API audit: `specs/done/2026/11/2026-11-21-04-terraform-provider-repo.md` and
`wiki/terraform-provider-api-audit.md` in the solidping repo.

## Layout
- `internal/client/`: thin handwritten HTTP client over the v1 REST API. No generated code.
- `internal/provider/`: provider, one file per resource, `data_sources.go`, shared `helpers.go`.
- `examples/`: HCL embedded in the docs. `docs/` is generated: never edit it by hand.

## Commands
| Command | Purpose |
|---|---|
| `make build` / `make lint` / `make test` | Compile, golangci-lint, unit tests |
| `make testacc` | Acceptance tests against a real server (README, "Developing") |
| `make docs` | Regenerate `docs/` after any schema or example change (CI fails when stale) |

## Rules
- Every resource needs an acceptance test covering create, import, update and destroy. The test
  framework checks that the plan is empty after each apply: a perpetual diff is a failing test.
- Check the API behaviour against a running server before modelling it, the OpenAPI file does not
  say everything (what a PATCH replaces or merges, which fields can be cleared and how).
- Server-defaulted attributes are `Optional + Computed` with `UseStateForUnknown`, never a static
  default copied from the server. Attributes the server rewrites on its own (check `regions`,
  section `position`) carry no `UseStateForUnknown`.
- An optional string the API cannot tell from empty goes through `optionalString`.
- Secrets: public keys in `config` / `settings`, secret keys in `*_sensitive`. See `refreshPublic`
  and `mergeSecrets` in `helpers.go` before touching either.
- A behaviour that depends on the server version is documented in the attribute description and
  its test step is guarded with `serverAtLeast`.
- Never name a real company in code, tests, examples or docs: use `acme` (`acme.com`,
  `alice@acme.com`).
- Never commit a token or a key. CI secrets: `GPG_PRIVATE_KEY`, `PASSPHRASE` (release only).
