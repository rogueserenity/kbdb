# Contributing

## Setup

Tool versions (Go, AWS CLI, SAM CLI, mockery, golangci-lint) are pinned via [mise](https://mise.jdx.dev/):

```sh
mise install
mise activate  # add to your shell profile so `go`, `sam`, `aws`, etc. resolve directly
```

`sam build` requires Docker running locally. On macOS with Docker Desktop, make sure `/Applications/Docker.app/Contents/Resources/bin` is on your `PATH` — otherwise `sam build`/`docker push` fail with a credential-store error.

## Everyday commands

```sh
mise run lint             # golangci-lint + actionlint + shellcheck
mise run test             # unit tests
mise run build            # sam build
mise run gen              # regenerate mocks, OpenAPI types, and go.mod/go.sum after changing an interface or api/openapi.yaml
mise run check-generated  # run gen and fail if it changed anything (what CI runs)
```

Single-test invocations use the underlying tools directly:

```sh
go test ./... -run TestVerifyTokenSuite -v   # a single suite
go test ./internal/auth/... -v               # a single package
sam validate --lint
```

## Running the app locally

```sh
mise run func-setup    # deploys template.yaml to floci + starts the WorkOS emulator
# export the KBDB_* vars func-setup prints, then:
mise run func-test     # runs the functional test suite against it
mise run func-teardown # tears it all down
```

`func-setup` deploys kbdb's real `template.yaml` to [floci](https://github.com/floci-io/floci) (a local AWS emulator) via a genuine `sam deploy` — not `sam local start-api`, which never emulates API Gateway's native JWT authorizer, so it couldn't exercise any auth-required write route (see `internal/middleware.RequireAuthorizerIdentity`). It prints `KBDB_*` exports (API URL, table names, auth tokens) each run — export those before `func-test`, since floci assigns real, stack-derived names rather than fixed local ones.

Most iteration just needs `mise run func-test` again once those are exported — Go code and test changes are picked up automatically. Restart (`mise run func-teardown && mise run func-setup`) after editing `template.yaml`, `docker-compose.floci.yml`, or the WorkOS emulator's seed config.

## Deploying to AWS

Deploys use a personal, isolated stack per developer rather than one shared environment, so nobody can break anyone else's testing.

```sh
mise run dev-setup     # one-time: creates your kbdb-dev-<name> stack
mise run dev-deploy    # deploy your current code to it
mise run dev-teardown  # tear it down completely, including its ECR images
```

`<name>` defaults to your `whoami`. Set `KBDB_DEV_NAME` to override it (e.g. if you want more than one stack, or your `whoami` collides with a teammate's).

These scripts derive your AWS account ID and region automatically from your active credentials (`aws sts get-caller-identity`, `aws configure get region`) — nothing to hardcode. Set `KBDB_DEV_REGION` to override the region if your CLI config doesn't set one.

You just need an active, authenticated AWS session — any profile works, there's no required profile name. This project's own maintainer setup happens to use a profile named `AWS_PROFILE=kbdb-dev-admin` (see [AWS accounts](#aws-accounts) below), but that's just this project's convention, not a requirement. **If you're forking this repo**, use any profile authenticated to your own AWS account, with either admin access or the [scoped dev policy](#giving-a-developer-scoped-access-no-admin-needed) attached.

### WorkOS OIDC config (`KBDB_OIDC_ISSUER_BASE_URL`/`KBDB_OIDC_AUDIENCE`)

All three scripts also require `KBDB_OIDC_ISSUER_BASE_URL` and `KBDB_OIDC_AUDIENCE` (the OIDC issuer and expected `aud` claim the deployed stack's JWT authorizer validates against). Get real values from the WorkOS dashboard's **Staging** environment (Configuration → issuer/AuthKit domain, and the User Management application's Client ID) — Staging is for dev/personal stacks only, never point a dev stack at WorkOS Production.

To avoid re-exporting these each session, copy `scripts/env/example-dev.env` to `scripts/env/<your-name>-dev.env`, fill it in, commit it (none of these values are secret), and symlink it: `ln -s <your-name>-dev.env scripts/env/dev.env`. All three scripts read that symlink if present; a real shell export still takes precedence.

### AWS accounts

Three accounts, one SSO session (`aws configure sso` once, then `aws sso login --profile <profile>` to refresh):

| Profile | Account | Purpose |
|---|---|---|
| `mgmt-admin` | `rogueserenity-management` | Administrative only, no workloads |
| `kbdb-dev-admin` | `kbdb-dev` | Hosts each developer's own stack |
| `kbdb-ci-admin` | `kbdb-ci` | CI's per-PR stacks |

There's no default AWS profile — commands will fail with `NoCredentials` unless `AWS_PROFILE` is set.

### First-time account bootstrap

New AWS account, never deployed to before? One-time steps, done once per account by whoever's setting it up (not needed for everyday `dev-deploy`). **If you're forking this repo to deploy to your own account, you only need step 1** — steps 3-5 exist for this project's own separate CI account and don't apply to a single-account personal deploy; `.github/workflows/` is entirely specific to this project's own CI and isn't something you need to set up or replicate.

1. **Artifact bucket** (needed by everyone): `aws cloudformation deploy --template-file bootstrap/artifact-bucket.yaml --stack-name kbdb-bootstrap --profile <profile>`. The scripts compute its name automatically (`kbdb-sam-artifacts-<your-account-id>`, matching this template's output) — nothing to copy into `samconfig.toml` by hand.
2. **ECR repo**: for a personal/`kbdb-dev`-style account, `mise run dev-setup` handles this automatically — nothing manual to do. For `kbdb-ci`'s shared bootstrap repo, see [ECR bootstrap procedure](#ecr-bootstrap-procedure) below.
3. **Cost budget** (only needed for accounts without their own app stack, e.g. `kbdb-ci`): `aws cloudformation deploy --template-file bootstrap/cost-budget.yaml --stack-name kbdb-cost-budget --profile <profile>`.
4. **(`kbdb-ci` only) GitHub Actions OIDC role**, so CI can authenticate to AWS: `aws cloudformation deploy --template-file bootstrap/ci-oidc-role.yaml --stack-name kbdb-ci-oidc --capabilities CAPABILITY_NAMED_IAM --profile kbdb-ci-admin`.
5. **(`kbdb-ci` only) JWKS bucket**, so CI's functional-test job can publish a real, publicly-fetchable issuer/JWKS for its per-PR stacks' native WorkOS authorizer to verify against: `aws cloudformation deploy --template-file bootstrap/jwks-bucket.yaml --stack-name kbdb-bootstrap-jwks --profile kbdb-ci-admin`. Then set the resulting `JWKSBucketName` output as the `JWKS_BUCKET_NAME` GitHub Actions repo variable (Settings → Secrets and variables → Actions → Variables) — `ci.yml`'s functional-test job fails at its "Publish emulator JWKS to S3" step until that's set.

After all bootstraps, ordinary `sam deploy` calls (and, for `kbdb-ci`, CI's own workflow) work.

#### ECR bootstrap procedure

The ECR repo needs to exist and hold an image *before* the first `sam deploy` can run, but `template.yaml` also declares that same repo as one of its own resources — a chicken-and-egg problem. `mise run dev-setup` automates this for `kbdb-dev`; for `kbdb-ci`'s one shared repo, do it by hand, once:

```sh
# 1. Create the repo standalone.
aws cloudformation deploy --template-file bootstrap/ecr-repo.yaml \
  --stack-name kbdb-ecr-bootstrap --profile <profile>

# 2. Build and push an initial image to it.
sam build --template-file template.yaml
sam package --s3-bucket kbdb-sam-artifacts-<account-id> \
  --image-repository <account-id>.dkr.ecr.<region>.amazonaws.com/kbdb-api \
  --output-template-file /tmp/packaged.yaml

# 3. Deploy the rest of the stack, excluding ApiRepository (it already
#    exists standalone; CloudFormation can't create a second one with the
#    same name). Use a temporary copy of template.yaml with the
#    ApiRepository resource block removed:
sam deploy --template-file <template-copy-without-ApiRepository>.yaml \
  --stack-name kbdb-dev --s3-bucket kbdb-sam-artifacts-<account-id> \
  --image-repositories ApiFunction=<account-id>.dkr.ecr.<region>.amazonaws.com/kbdb-api \
  --capabilities CAPABILITY_IAM --no-confirm-changeset

# 4. Import the standalone repo into the now-existing stack, so
#    template.yaml "owns" it going forward.
aws cloudformation create-change-set --stack-name kbdb-dev \
  --change-set-name import-ecr-repo --change-set-type IMPORT \
  --template-body file:///tmp/packaged.yaml --capabilities CAPABILITY_IAM \
  --resources-to-import '[{"ResourceType":"AWS::ECR::Repository","LogicalResourceId":"ApiRepository","ResourceIdentifier":{"RepositoryName":"kbdb-api"}}]'
aws cloudformation execute-change-set --stack-name kbdb-dev --change-set-name import-ecr-repo
```

A CloudFormation `IMPORT` changeset can only import into a stack that already exists, which is why step 3 runs before step 4 rather than combined with it.

### Giving a developer scoped access (no admin needed)

Once the account is bootstrapped, day-to-day `dev-setup`/`dev-deploy`/`dev-teardown` don't need admin access — a much narrower policy covers exactly what those three scripts do. This is the setup for a fork you're deploying to your own AWS account, or for adding a teammate without handing them broad permissions:

1. **Account admin, once**: deploy the scoped policy.
   ```sh
   aws cloudformation deploy --template-file bootstrap/dev-user-policy.yaml \
     --stack-name kbdb-dev-user-policy --capabilities CAPABILITY_NAMED_IAM --profile <admin-profile>
   ```
   Attach the resulting policy (output as `DevPolicyArn`) to each developer's IAM user, group, or role:
   ```sh
   aws iam attach-user-policy --user-name <dev> --policy-arn <DevPolicyArn>
   ```
2. **Each developer**: with that policy attached (and no other permissions needed), run `mise run dev-setup`, `dev-deploy`, `dev-teardown` as usual.

This policy is scoped to `kbdb-dev-*`-named stacks/resources only — it can't touch anything outside a developer's own stack, and can't grant itself broader access. It was verified by running the full `dev-setup` → `dev-deploy` → `dev-teardown` cycle as a real IAM user with only this policy attached and nothing else.

## Testing strategy

Two test layers, kept separate — don't mix them:

- **Unit tests** (`*_test.go` next to the code they test): `testify/suite` + `mockery`-generated mocks. No real infra, no network calls.
- **Functional tests** (`test/functional/features/`): Ginkgo + Gomega, driving real HTTP/MCP calls against a running stack (local via `func-setup`, or a real deployed stack via `KBDB_API_BASE_URL`).

Functional specs follow a consistent BDD shape: `Describe` the subject (and, for multi-action resources, a nested `Describe` per action) → `Context("given <precondition>")` → `When("<action>")` → one `It` with a `By(...)` per assertion. Build the actual request inside the innermost `BeforeEach`, not an outer one — Ginkgo's context is scoped to the node that received it.

The WorkOS emulator (`ghcr.io/workos/emulate`) stands in for WorkOS in functional tests, since `auth.NewVerifier` does a real OIDC discovery round-trip and needs something real to talk to. It runs as its own docker-compose service, seeded via `scripts/workos-emulate-seed.yaml`.

## Conventions

- **Commit messages and PR titles** follow [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): subject` (e.g. `fix(ci): scope Cognito IAM permissions to account/region`). Common types: `feat`, `fix`, `chore`, `docs`, `test`, `ci`, `refactor`.
- **Mise tasks live in `scripts/`**, one `.sh` file per task, referenced from `mise.toml` via `file = "scripts/<name>.sh"` — even one-liners. Add a new task the same way.
- Package layout, mocking patterns, and other code-level conventions are documented in `CLAUDE.md`.
