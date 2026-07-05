# Contributing

## Commands

Common dev actions run through `mise run <task>` (aliases in parentheses) rather than raw tool invocations directly — one consistent entrypoint regardless of what's underneath:

```sh
mise run lint          # (l)  golangci-lint run ./... && actionlint
mise run test          # (t)  go test on unit tests only (excludes test/functional)
mise run build         # (b)  sam build
mise run dev-setup     # (ds) one-time per-developer kbdb-dev-<name> stack bootstrap (requires AWS_PROFILE=kbdb-dev-admin)
mise run dev-deploy    # (dd) deploy to your personal kbdb-dev-<name> stack (requires AWS_PROFILE=kbdb-dev-admin; run dev-setup first)
mise run dev-teardown  # (dt) delete your personal kbdb-dev-<name> stack, including its owned ECR repo
mise run func-setup    # (fs) bring up LocalStack + mockoidc (docker compose) + sam local start-api
mise run func-test     # (fx) run functional tests (Ginkgo) against whatever func-setup brought up
mise run func-teardown # (ft) tear down whatever func-setup started
```

**`kbdb-dev` supports multiple concurrent per-developer stacks**, not one shared stack: each developer gets their own `kbdb-dev-<name>` CloudFormation stack, with `<name>` derived from `whoami` by default (override via the `KBDB_DEV_NAME` env var — useful for testing multiple concurrent stacks as one person, or disambiguating two developers who'd otherwise collide under the same `whoami` result). Each stack fully owns its own resources, including its own ECR repo (`kbdb-api-kbdb-dev-<name>`, not shared) — this was a deliberate choice over one repo shared across developers specifically so `dev-teardown` can delete a developer's entire stack, including their image history, as a single unit with nothing left to separately clean up. `dev-setup` automates the ECR chicken-and-egg bootstrap dance (see below) end-to-end for a new developer; it refuses to run if the target stack already exists (points at `dev-teardown` instead of attempting to detect/resume partial state).

`func-setup`/`func-teardown` are separate from `func-test` because they have different lifecycles: setup is long-running local infra you leave running, func-test is a one-shot command repeatedly run against it (also the command CI invokes, just against a different `KBDB_API_BASE_URL`).

**After fixing a bug and wanting to retest, whether you need to restart `func-setup` depends on what changed**:
- Go app code (`internal/`, `functions/api/`) or the Ginkgo specs/support code (`test/functional/`) — no restart needed. `sam local start-api` rebuilds the Lambda image on the next invocation automatically, and Ginkgo compiles fresh on every `func-test` run. Just `mise run func-test` again.
- `template.yaml`, `docker-compose.yml`, or mockoidc's own source (`test/functional/support/mockoidc/`) — these are only picked up by `func-setup`/`func-teardown`, not by re-running `func-test`. `mise run func-teardown && mise run func-setup` (~20-25s total, mostly Docker Compose network churn + `sam build`'s fixed overhead — not worth splitting into finer-grained tasks at that cost).

Single-test/package invocations still use the underlying tools directly, not a mise task:
```sh
go test ./... -run TestVerifyTokenSuite -v   # single suite
go test ./internal/auth/... -v               # single package
mockery                                       # regenerate mocks after changing an interface in internal/auth (or adding new ones to .mockery.yml)
sam validate --lint
```

## AWS account structure

**The AWS Organization is three accounts**, each with its own SSO profile (`~/.aws/config`, same `kbdb` SSO session): `mgmt-admin` (`rogueserenity-management`, 957814222990 — administrative-only, no workloads), `kbdb-dev-admin` (`kbdb-dev`, 992234857260 — hosts each developer's own `kbdb-dev-<name>` stack, no bare `kbdb-dev` stack), `kbdb-ci-admin` (`kbdb-ci`, 475976462467 — CI's per-PR stacks). Deploys use IAM Identity Center (SSO) profiles, not default credentials or long-lived keys — set up once via `aws configure sso`, refreshed via `aws sso login --profile <profile>` when the session expires. There is no default/implicit profile wired up; commands against real AWS will fail with `NoCredentials` without `AWS_PROFILE=<profile>` (or `--profile <profile>`). Centralized root access management (`RootCredentialsManagement`/`RootSessions`) is enabled at the Org level — `kbdb-dev`/`kbdb-ci` have no standalone root password; privileged root-only actions go through the management account's IAM > Root access management console instead.

`sam build` requires Docker running locally — `ApiFunction` is packaged as a container image, not a zip. On macOS with Docker Desktop, `docker-credential-desktop` must be on `PATH` or `sam build`/`docker push` fail with a credential-store error; add `/Applications/Docker.app/Contents/Resources/bin` to `PATH` if so.

**First deploy to a new AWS account/stack**: three one-time bootstrap steps (plus a fourth, `kbdb-ci`-only step below), none of which `sam deploy` can do for itself. All `bootstrap/*.yaml` templates are deployed via plain `aws cloudformation deploy`, not `sam deploy` (they're bootstrapping the things `sam deploy` itself depends on).

1. **The S3 artifact bucket**:
   ```sh
   aws cloudformation deploy --template-file bootstrap/artifact-bucket.yaml --stack-name kbdb-bootstrap --profile <account-profile>
   ```
   An explicit, version-controlled bucket (with a 7-day noncurrent-version-expiration lifecycle policy) rather than SAM's own `--resolve-s3`/auto-managed bucket, specifically so that lifecycle policy can exist at all — SAM's auto-managed bucket has no template to attach one to, and its default versioning-enabled-with-no-lifecycle-rule setup otherwise accumulates old object versions/delete markers forever. Update `samconfig.toml`'s `s3_bucket` with the new account's bucket name (`kbdb-sam-artifacts-<account-id>`, from this template's `ArtifactBucketName` output).

2. **The ECR repo** — the repo must exist and hold an image before `sam deploy` can resolve `ApiFunction`'s `ImageUri`, but `template.yaml` also declares that same repo as a CloudFormation resource in the same stack. Genuine chicken-and-egg, no first-party SAM fix. **For `kbdb-dev` specifically, this is automated end-to-end by `mise run dev-setup`** (each developer owns their own repo, per the per-developer-stack design above) — read `scripts/dev-setup.sh` for the exact scripted steps if modifying that automation. The manual procedure below is still how `kbdb-ci`'s one shared, account-level bootstrap repo (used by CI's per-PR stacks, which don't own their own repo) is set up — a one-time-per-account operation, not per-developer:
   ```sh
   # a. Create the repo standalone (bootstrap/ecr-repo.yaml has DeletionPolicy: Retain
   #    baked in from the start - required before it can later be imported).
   aws cloudformation deploy --template-file bootstrap/ecr-repo.yaml --stack-name kbdb-ecr-bootstrap --profile <account-profile>

   # b. Push an initial image to it. sam build/sam package must be pointed at a
   #    Metadata.DockerContext-resolvable template - if using a temporary
   #    template copy with ApiRepository removed (step c), that copy MUST live
   #    inside the repo root (not /tmp or elsewhere), since DockerContext: .
   #    resolves relative to the template file's own location, not the repo.
   sam build --template-file template.yaml
   sam package --s3-bucket kbdb-sam-artifacts-<account-id> \
     --image-repository <account-id>.dkr.ecr.<region>.amazonaws.com/kbdb-api \
     --output-template-file /tmp/packaged.yaml

   # c. Create the REST of the stack (everything except ApiRepository, since it
   #    already exists standalone and CloudFormation can't create a second,
   #    colliding repo of the same name). Temporarily deploy a copy of
   #    template.yaml with the ApiRepository resource block removed:
   sam deploy --template-file <template-copy-without-ApiRepository>.yaml \
     --stack-name kbdb-dev --s3-bucket kbdb-sam-artifacts-<account-id> \
     --image-repositories ApiFunction=<account-id>.dkr.ecr.<region>.amazonaws.com/kbdb-api \
     --capabilities CAPABILITY_IAM --no-confirm-changeset

   # d. Import the standalone repo into the now-existing stack. Add
   #    DeletionPolicy: Retain to ApiRepository in the PACKAGED template first
   #    (a hard CloudFormation import prerequisite - it refuses to import any
   #    resource without one already present in the template being submitted):
   aws cloudformation create-change-set --stack-name kbdb-dev \
     --change-set-name import-ecr-repo --change-set-type IMPORT \
     --template-body file:///tmp/packaged.yaml --capabilities CAPABILITY_IAM \
     --resources-to-import '[{"ResourceType":"AWS::ECR::Repository","LogicalResourceId":"ApiRepository","ResourceIdentifier":{"RepositoryName":"kbdb-api"}}]'
   aws cloudformation execute-change-set --stack-name kbdb-dev --change-set-name import-ecr-repo
   ```
   A CloudFormation `IMPORT`-type changeset can **only** import resources into a stack that already exists (or where every resource in the template is either being imported or already unchanged) — it cannot simultaneously create N new resources and import one in a single operation, which is why step (c) has to happen before (d), not combined with it.

3. **The cost budget** (only for accounts with no app stack of their own, e.g. `kbdb-ci`, `rogueserenity-management` — `kbdb-dev` already gets one from `template.yaml`'s own `CostBudget` resource):
   ```sh
   aws cloudformation deploy --template-file bootstrap/cost-budget.yaml --stack-name kbdb-cost-budget --profile <account-profile>
   ```
   Mirrors `template.yaml`'s `CostBudget` resource so every account has this tripwire regardless of whether it hosts a real app stack — parameterized by `BudgetLimitUsd`/`AlertEmail` if an account's threshold should differ from the defaults.

4. **(`kbdb-ci` only) The GitHub Actions OIDC provider + deploy role** — `bootstrap/ci-oidc-role.yaml` declares the `AWS::IAM::OIDCProvider` (trusting `token.actions.githubusercontent.com`) and the `kbdb-github-actions-deploy` role `.github/actions/kbdb-pr-stack` assumes, with its trust policy scoped to the exact `sub` claim `repo:rogueserenity/kbdb:pull_request` (verified against a real token, not just documentation, before committing to it) and a permissions policy scoped to `kbdb-pr-*` resource patterns everywhere that's possible (CloudFormation, Lambda, IAM roles, the ECR repo, Cognito — pool IDs are always region-prefixed, e.g. `us-east-2_XXXXXXXXX`, so Cognito is scoped to this account+region's pools even though a specific PR's pool ID isn't known ahead of time) — API Gateway v2's management-plane actions are the one exception that must stay scoped only to `/apis/*` and `/tags/*` path prefixes rather than a specific API ID, since those IDs are opaque and don't exist until after a PR's own stack is created, not a scoping oversight. Neither the OIDC provider nor the role can be created by CI itself (CI has no credentials until this trust relationship exists), so — like the other three — it's deployed once via `aws cloudformation deploy --template-file bootstrap/ci-oidc-role.yaml --stack-name kbdb-ci-oidc --capabilities CAPABILITY_NAMED_IAM --profile kbdb-ci-admin` by a human operator.

After all bootstraps, ordinary `sam deploy` calls (and, for `kbdb-ci`, the `ci.yml` CI workflow) work.

**`samconfig.toml` is committed**, not gitignored — nothing in it is a secret (AWS account IDs, stack names, and ECR repo URIs are not sensitive; only IAM credentials would be, and none are stored here). `[default.deploy.parameters]` holds only truly shared, non-stack-specific `kbdb-dev` values (region, capabilities) — `stack_name`/`image_repositories` are deliberately NOT set there, since `kbdb-dev` supports multiple concurrent per-developer stacks (see above); `mise run dev-setup`/`dev-deploy`/`dev-teardown` compute and supply those per-run instead. **A second environment (a real prod stack, when one exists) should be added as a new named section** (e.g. `[prod.deploy.parameters]`) selected via `sam deploy --config-env prod` — SAM's actual supported mechanism for multiple environments in one file. Note: SAM does **not** support `${VAR}`-style interpolation inside `samconfig.toml` values (confirmed by testing) — don't reach for that; use `--config-env` and real literal values per named section instead.

## Testing strategy

Two frameworks, split by test type — not interchangeable, don't mix them:
- **Unit tests**: `testify/suite` + `mockery`-generated mocks, colocated with the code (`foo_test.go` next to `foo.go`). For function-level logic with dependencies injected via interfaces — no real infra, no network calls.
- **Functional/black-box tests**: Ginkgo v2 + Gomega, living under a centralized `test/functional/features/{api,mcp}/` tree (not colocated), driving real HTTP/MCP clients against a real running stack — locally via `mise run func-setup` (LocalStack + `mockoidc` via `docker-compose.yml`, plus `sam local start-api`), or a real deployed stack (set `KBDB_API_BASE_URL`; CI points at a real ephemeral per-PR AWS stack). Gomega's `ghttp` (HTTP mocking) is explicitly not used here — the point of this layer is exercising the real deployed thing, not mocks. The Ginkgo CLI is pinned via `go.mod`'s `tool` directive (`go tool ginkgo run ./test/functional/...`, or `mise run func-test`), not a mise-managed plugin.
- **Functional specs follow a strict BDD structure — don't deviate.** One file per resource/action group (`ping_test.go`, `echo_test.go`, later `keyboards_test.go`, etc.). One top-level `Describe("<Subject>")` per file; for a resource with multiple distinct actions (e.g. Keyboards' Get/List/Create/Update/Delete), add a second-level `Describe("<Action>")` per action — collapse this level only when a file genuinely has just one action (`ping`, `echo`). Below that: `Context("given <precondition>")` (its `BeforeEach` sets up that precondition) → `When("<action taken>")` (its `BeforeEach` performs the action, capturing the result) → a single `It("<outcome>")` with one `By("<specific Then>")` per assertion, even if there's only one. `DescribeTable`/`Entry` is fine for genuine data-driven variations but isn't the default shape. **Build the actual request/call (and its context) in the innermost action-performing `BeforeEach`, never in an outer one** — Ginkgo's `SpecContext` is scoped to the node invocation that received it, so an `*http.Request` built with an outer `BeforeEach`'s `ctx` fails with `"spec has finished"` once a nested `BeforeEach` tries to use it; outer `BeforeEach`es should only stage plain data (tokens, headers) for the innermost one to assemble into the real request.
- **`mockoidc`** (`github.com/oauth2-proxy/mockoidc`) stands in for Cognito for functional tests — required, not optional, because `auth.NewVerifier` calls `oidc.NewProvider`, which does a real HTTP OIDC discovery round-trip; the functional-test layer has no injection seam (unlike unit tests, which mock the `tokenVerifier` interface directly), so whatever plays the issuer role must be a real, spec-compliant OIDC server. Runs as its own docker-compose service (`test/functional/support/mockoidc/`, built from its own `Dockerfile`) rather than a bare local process, so `sam local start-api`'s Lambda container can reach it via a stable Docker-network service name instead of host-networking workarounds. **`mockoidc.NewServer`'s default `Server.Addr` (derived from the listener's bind address) is not a usable issuer hostname for any client outside that process's network namespace** — it must be overwritten (`m.Server.Addr = ...`) with the address other containers actually use to reach it *before* anything calls `Issuer()`/serves discovery, or every signed JWT's `iss` claim and the discovery document will carry an unreachable address (see `test/functional/support/mockoidc/main.go`'s `MOCKOIDC_ADVERTISE_ADDR`). Test client ID/secret/user subject are fixed constants (`test/functional/support/mockoidc/fixtures/`), not `mockoidc.NewServer`'s randomly-generated defaults, so specs and the local `sam local start-api` env-vars file can reference known values without an out-of-band discovery step.

Test data isolation (once functional tests exist): a fresh synthetic `user_id` (UUID) generated per spec, not table truncation — every table is partitioned by `user_id`, so parallel specs can never collide even sharing the same DynamoDB table.

## Toolchain

Go, `aws-cli`, `aws-sam-cli`, `mockery`, and `golangci-lint` versions are pinned in `mise.toml` — run `mise install` once, then `mise activate` (already wired into most shells via `.zshrc`/`.bashrc`) makes `go`/`sam`/`aws`/`mockery`/`golangci-lint` resolve directly without a `mise exec --` prefix. If a Bash tool session doesn't have `mise activate` in effect (e.g. non-interactive subshells), fall back to `mise exec -- <command>`. The Ginkgo CLI is the one exception — pinned via `go.mod`'s `tool` directive instead of `mise.toml`, invoked as `go tool ginkgo ...` (see Testing strategy above).

**Every `mise.toml` task, including one-liners, is a real script in `scripts/`, referenced via `file = "scripts/<name>.sh"`**, not embedded inline as a `run = """..."""` string — mise supports both, but a real standalone `.sh` file gets normal shell tooling (editor syntax highlighting, `shellcheck` runs directly against it without extraction) that an inline TOML string block doesn't, and keeps `mise.toml` itself a short, scannable index of tasks rather than a mix of index and implementation. `mise run lint` includes an explicit `shellcheck scripts/*.sh` pass for this reason (`actionlint`'s own shellcheck integration only covers workflow files' inline `run:` blocks, not standalone scripts). Add new tasks the same way: write `scripts/<name>.sh`, then point a `[tasks.<name>]` entry at it.

**Local dev environment** (`docker-compose.yml`, `mise run func-setup`/`func-teardown`): brings up LocalStack and `mockoidc` as docker-compose services. LocalStack requires a free account + `LOCALSTACK_AUTH_TOKEN` env var (sign up at localstack.cloud) — its no-signup "Community edition" was deprecated, so this is required even though LocalStack isn't yet exercised by any application code (no DynamoDB tables exist until Phase 1). It was set up ahead of that need anyway: deferring wouldn't reduce the eventual cost of the signup/token requirement, and standing it up now means it's a known-working part of the dev loop before Phase 1 adds real pressure.

## Commit messages

Commit messages and PR titles follow [Conventional Commits](https://www.conventionalcommits.org/) (`type(scope): subject`, e.g. `fix(ci): scope Cognito IAM permissions to account/region`). Common types: `feat`, `fix`, `chore`, `docs`, `test`, `ci`, `refactor`. Scope is the affected area (e.g. `ci`, `auth`, `mcp`) and can be omitted when a change is broad or scope isn't meaningful. This applies going forward from when it was adopted — existing commit history predating it was not retroactively rewritten.
