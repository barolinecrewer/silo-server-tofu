# AGENTS.md

Working agreement for coding agents (and humans) in this repository. Read this
before making changes. When CLAUDE.md and this file disagree, this file wins.

## What this repo is

An OpenTofu provider for [Silo](https://github.com/Silo-Server) — a
self-hosted media server — written in Go on `terraform-plugin-framework`
(protocol 6). The provider manages **server configuration as infrastructure
as code** through the Silo `/api/v2` HTTP API: accounts, access groups,
libraries, collections, API keys, invite codes, settings, autoscan,
notifications, request routing, and similar administrative surfaces.

**OpenTofu first.** The provider is developed, tested, and documented
against the OpenTofu CLI (`tofu`). It also runs on Terraform 1.7+ because
both CLIs speak plugin protocol 6 — but when the two differ, tofu wins:
CI validates examples with the real `tofu` binary, acceptance tests default
to `tofu` via `TF_ACC_TERRAFORM_PATH`, docs say `tofu` before `terraform`,
and the registry address is `registry.opentofu.org`. The `terraform {}`
configuration block name is kept because tofu accepts it for compatibility.

Non-goals:

- **User runtime state is not managed.** Watch progress, playback sessions,
  ratings, and favorites are out of scope as resources. Runtime surfaces may
  appear as read-only data sources only when there is a real config use case.
- **Compatibility endpoints are out of scope.** Jellyfin and Audiobookshelf
  (Beta) compatibility endpoints follow their own contracts and are not part
  of the v2 API contract this provider targets.
- **Beta features are excluded until they graduate.** The reference marks
  endpoints that may change or be removed; do not build resources on them.

## Ground rules

1. **The live API reference is the source of truth.** Each Silo server
   documents the API it runs at `<server>/api/v2/openapi.json` (OpenAPI 3;
   ~619 paths, ~1034 schemas, several MB). Every request body, response
   schema, query parameter, header, and status code must be verified against
   it before writing code. Never guess field names or shapes. Do not load
   the document into context wholesale — fetch and query it.

2. **Fetch the reference with `curl`, query it with `jq` or `python3`, never
   read it.** Useful entry points:

   ```sh
   # Fetch the live contract once per task (public; no sign-in needed)
   curl -sL <server>/api/v2/openapi.json -o /tmp/silo-openapi.json

   # All endpoints for a domain
   jq -r '.paths | keys[]' /tmp/silo-openapi.json | grep access-groups

   # One endpoint's full contract (body, responses, params, headers)
   jq '.paths["/api/v2/admin/access-groups/{id}"].put' /tmp/silo-openapi.json

   # A response/request schema
   jq '.components.schemas.AdminAccessGroup' /tmp/silo-openapi.json

   # Where a schema is referenced
   jq -r '[.paths | to_entries[] | .key as $p | .value | to_entries[]
     | select(.. | scalars as $v | select($v == "#/components/schemas/AdminAccessGroup")) | $p]
     | unique | .[]' /tmp/silo-openapi.json
   ```

3. **The contract is not vendored in this repo.** The document is large and
   versioned with the server; the provider targets the stable `/api/v2`
   contract shapes. Note the server version a document came from when
   implementing against it. If a live document differs from what an
   implementation expects, open an issue comparing the two documents instead
   of silently coding against a different shape.

4. **Interactive reference for exploration:** `<server>/api/v2/docs` needs no
   sign-in. Requests do. The public usage guide is
   <https://siloserver.org/docs/api>.

## Scaffold layout

```
silo-server-tofu/
├── AGENTS.md                  this file
├── CLAUDE.md                  symlink to this file
├── README.md                  project overview + scaffold diagram
├── main.go                    provider entry point (providerserver)
├── go.mod / go.sum            module github.com/barolinecrewer/silo-server-tofu
├── justfile                   build / test / testacc / tofu-validate / lint / generate
├── .golangci.yml              starter lint config
├── .github/workflows/ci.yml   build + vet + test on push/PR
├── internal/
│   ├── client/                the ONLY package that talks HTTP
│   │   ├── client.go          Client core: auth, ETag capture, retries
│   │   ├── errors.go          *APIError + status classifiers
│   │   └── access_groups.go    one file per API domain: models + calls
│   └── provider/              terraform-plugin-framework surfaces
│       ├── provider.go        Metadata, schema, Configure, registries
│       ├── provider_test.go   offline schema/metadata tests
│       ├── values.go          shared tfsdk ⇄ Go value conversions
│       ├── access_group_resource.go        pattern reference resource
│       ├── access_group_resource_test.go   acceptance test (TF_ACC-gated)
│       ├── access_groups_data_source.go    pattern reference data source
│       └── ...               one file pair per resource/data source
├── examples/                  runnable examples; doubles as docs source
│   ├── provider/provider.tf
│   ├── resources/silo_access_group/resource.tf
│   └── data-sources/silo_access_groups/data-source.tf
└── tools/tools.go             build-time tooling pins (tfplugindocs)
```

Layering is strict and one-directional:

```
main.go → internal/provider → internal/client → Silo /api/v2
```

The `provider` package maps tfsdk values to Go models. The `client` package
owns HTTP, auth, error mapping, ETags, and pagination. Nothing in `client`
imports `provider`; nothing in `provider` imports `net/http`.

## API contract essentials

Implement to these rules everywhere; they are the difference between a
provider that works and one that corrupts state.

- **Auth:** `Authorization: Bearer <api-key>` on every request. An API key
  carries its owner's permissions; admin resources need an admin-owned key.
  Never put keys in URLs. In config, `api_key` is `Sensitive`.
- **IDs are strings.** Opaque, never integers in Go models or state. Some are
  numeric-looking, some are not; never parse them.
- **ETag-guarded writes.** `PUT` and `DELETE` on guarded resources require
  `If-Match: <etag>`; a missing header is `428`, a stale tag is `412` with the
  current ETag in the response. The provider stores the read-time `ETag`
  response header in state (`etag` computed attribute) and sends it on writes.
  `"*"` means overwrite deliberately — use it only as a deliberate escape
  hatch, never by default.
- **Cursor pagination.** List endpoints take `limit` (default 50, max 200) and
  an opaque `cursor`; responses carry `page.has_more` and `page.next_cursor`.
  Pass cursors back unchanged. Never count pages or assume ordering.
- **Capability-first.** Most domains expose `.../capabilities` returning
  `state` (`available` / `disabled` / `not_configured` / `unsupported`) and
  `allowed`. Read the capability before offering a feature: a `disabled`
  feature is different from a failed request. Resources whose capability says
  `unsupported` should surface a clear diagnostic, not a 403 mystery.
- **Error semantics.** Map status codes consistently:
  - `401` — credential problem (config error, fail fast)
  - `403` — the key's owner/scopes lack permission, or a profile needs
    verification (config error, fail with a clear diagnostic)
  - `404` on Read — remove the resource from state; on Delete — treat as gone
  - `412` / `428` — precondition failure: someone changed the resource outside
    Terraform; fail the operation with a message telling the user to refresh
    or import — never silently overwrite
  - `422` — validation; surface the problem body detail
  - `429` — rate limited; follow the response's retry instructions, back off,
    and retry with a bounded count on **reads only**
  - `5xx` — server problem; fail with the request ID if present
- **Never auto-retry writes.** If the connection drops during a `POST`, the
  write may already have succeeded; retrying can duplicate credentials,
  invite codes, or accounts. Surface the ambiguity to the user.
- **Profiles are out of scope** for the provider's config plane. `X-Profile-Id`
  / `X-Profile-Token` are user-session concepts; do not add them to provider
  config. (If a future need arises, it goes through the checklist below.)

## Provider design conventions

- **Protocol 6, `terraform-plugin-framework` only.** No SDKv2, no
  `helper/schema`, no `schema.Resource`. Ever.
- **Provider type name is `silo`**; resource names are `silo_<singular>`
  (`silo_access_group`), data sources `silo_<plural or noun>`
  (`silo_access_groups`).
- **Registry address:** `registry.opentofu.org/silo-server/silo`.
- **All IDs are `types.String`.** State `id` is the API's opaque ID.
  `ImportStatePassthroughID` on every resource.
- **Response-derived fields are `Optional: true, Computed: true`.** The API
  fills in defaults (e.g. `max_streams: 0`, `download_allowed: false`), so
  write bodies are built from pointers: **unknown or null plan values are
  omitted from create bodies; update bodies carry the full desired state**
  (computed values come from prior state). Mirror `AdminXBody` write schemas
  exactly — including which fields are nullable.
- **Server-only fields (`created_at`, `updated_at`, `member_count`, `etag`)
  are `Computed` only.**
- **Every example directory is self-contained.** It declares
  `required_providers` with `source = "silo-server/silo"` — a bare `silo`
  without the block resolves to the `hashicorp/` namespace and fails — and
  it must pass `tofu fmt -check` and `tofu validate` (run via dev_overrides
  with `just tofu-validate`).
- **Response-only pagination totals are never state.** If a list response has
  a separate `total`, do not pin it in state.
- **One file per resource + one per data source**, named after the type
  (`access_group_resource.go`). The models/calls live in `internal/client`
  in a domain file (`access_groups.go`). Keep the mapping direction: client
  models are plain Go + `encoding/json`; only `provider` knows about tfsdk.
- **Diagnostics, not panics or `log.Fatal`.** Use
  `resp.Diagnostics.AddError(summary, detail)`; summaries are stable
  ("API error creating access group"), details carry the dynamic parts.
- **No `context.TODO()`.** Thread the framework-provided context everywhere.

## Adding a resource or data source

Follow the `access_group` pattern exactly. The checklist:

1. **Verify the contract.** Pull every operation's path, body, response,
   params, and headers from the live reference (see ground rules for
   jq snippets). Note the capability endpoint and the paginated list shape.
2. **Client first.** Add the domain file in `internal/client`: write-body
   struct (pointers, `omitempty`), response struct, and one method per
   operation. Methods return `(model, etag, error)` when the endpoint carries
   ETags. Add path constants. Unit-test JSON marshalling of the write body.
3. **Resource/data source file** in `internal/provider`. Copy the schema
   conventions above; implement `Metadata`, `Schema`, `Configure`,
   `Create`, `Read`, `Update`, `Delete`, `ImportState` (resources) or
   `Read` (data sources).
4. **Register it** in `provider.go` (`Resources` / `DataSources`).
5. **Examples.** One runnable example under
   `examples/resources/<type>/resource.tf` (or `examples/data-sources/...`),
   using `testacc`-safe values.
6. **Tests.**
   - Offline unit tests for model mapping and body building.
   - Acceptance test in `<file>_test.go` named `TestAcc<Type>`, gated by
     `TF_ACC` and env config (see testing policy). Cover create, update
     (verify `If-Match` flow), import, and delete.
7. **Docs.** Update `README.md` coverage table; `tfplugindocs` templates pick
   up the example when docs generation is wired.
8. **Gates.** `go build ./...`, `go vet ./...`, `go test ./...` clean;
   acceptance tests pass against a throwaway server.

## Adding an API domain

Same as above, but first classify it from the reference:

- **Config plane** (admin CRUD with ETags) → resources: accounts, access
  groups, libraries, collections, API keys, invite codes, settings, autoscan
  sources, notification channels, request routing, sections, plugins.
- **Server introspection** (versions, capabilities, status) → data sources:
  `silo_capabilities`, `silo_server`, etc.
- **Runtime/user plane** (playback, progress, watch-together, favorites) →
  out of scope; document the decision in the PR if it ever comes up.
- **Jobs/tasks/scans** (async, not declarative) → not resources. If ever
  exposed, they become data sources with clear caveats.

## Testing policy

- **Offline tests run by default** (`go test ./...`): schema validity, model
  mapping, body marshalling, error classification. No network, no server.
- **Acceptance tests are opt-in.** `TestAcc*` requires `TF_ACC=1` plus:
  - `SILO_ACC_BASE_URL` — base URL of a **throwaway** test server
  - `SILO_ACC_API_KEY` — an admin-owned API key on that server
  They create and destroy real resources. **Never point them at a production
  server** or at an instance with real libraries; they are destructive.
- **Acceptance tests run the OpenTofu CLI.** The test harness picks its CLI
  from `TF_ACC_TERRAFORM_PATH`; `just testacc` wires it to `tofu` on PATH
  (falling back to `terraform`). Version checks (`tfversion.SkipBelow`)
  work with tofu because `tofu version -json` reports a `terraform_version`
  field the harness parses.
- **PreCheck skips, not fails**, when env vars are missing, so plain
  `go test` stays green without a server.
- **No golden-file tests of the OpenAPI document** — its size would make
  diffs useless; test the client against focused fixtures instead.

## Commands

```sh
just build          # go build -o bin/terraform-provider-silo .
just test           # offline unit tests (no server needed)
just testacc        # acceptance tests; runs the tofu CLI, needs TF_ACC env
just tofu-validate  # build + `tofu validate` every example via dev_overrides
just lint           # golangci-lint + `tofu fmt -check` on examples
just generate       # regenerate provider docs from templates (tfplugindocs)
```

Acceptance run (the tofu CLI is wired automatically):

```sh
TF_ACC=1 SILO_ACC_BASE_URL=http://localhost:8090 \
  SILO_ACC_API_KEY=<test-key> just testacc
```

## Code style

- Standard Go: gofmt-clean, `go vet` clean. Keep comments about *why*,
  not *what*; no commented-out code.
- Error wrapping: `fmt.Errorf("...: %w", err)`.
- `internal/client` stays dependency-free beyond stdlib (+ tests). Framework
  imports never appear below `internal/provider`.
- No new top-level packages without updating this file's layout section.

## Docs

- `README.md` holds the coverage roadmap — keep the status column honest.
- Examples double as documentation source for `tfplugindocs`
  (`templates/` when wired). Example configs must be copy-paste runnable.
- The Silo name and mark are trademarks of Silo Media L.L.C.; this repo is
  referential use ("provider for Silo"). Do not brand a fork as Silo.
