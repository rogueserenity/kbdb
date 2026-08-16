# RETRACTED — issue 04 was never a floci bug

The original version of this file claimed floci's `GetAuthorizationToken`
returned an unreachable AWS-shaped ECR hostname, breaking `sam deploy` for
image-backed functions. That was wrong.

## What was actually wrong

floci was never the problem. `EcrRegistryManager.getProxyEndpoint()` and
`getRepositoryUri()` both correctly return a reachable host —
`http://localhost:5100` and (with the default `hostname` URI style)
`{account}.dkr.ecr.{region}.localhost:5100`, which resolves to `127.0.0.1`.
Verified directly:

```
$ aws ecr get-authorization-token --query 'authorizationData[0].proxyEndpoint' --output text
http://localhost:5100

$ aws ecr describe-repositories --query 'repositories[].repositoryUri' --output text
000000000000.dkr.ecr.us-east-2.localhost:5100/kbdbflociecrtesta1249a8e/apifunction51503098repo
```

The actual failing request went to `.amazonaws.com`, not `.localhost:5100`.
`sam deploy --resolve-image-repos` was diagnosed with `--debug`:

```
The push refers to repository [000000000000.dkr.ecr.us-east-2.amazonaws.com/...]
```

**`sam deploy --resolve-image-repos` fabricates the standard AWS ECR
hostname pattern itself.** It never reads floci's `repositoryUri` from
`CreateRepository`'s response. This is a SAM CLI behavior, not something
floci returns or controls, and it reproduces identically regardless of
floci's `services.ecr.uriStyle` setting (`hostname` or `path`).

## What actually works

Every real deploy path in this repo already avoids `--resolve-image-repos`
in favor of an explicit `--image-repositories`
(`scripts/dev-deploy.sh`, `.github/workflows/ci.yml`) — because real AWS
has the same "which repo do I push to" ambiguity `--resolve-image-repos` is
meant to paper over, just with a different failure mode there.

Doing the same against floci, with `services.ecr.uriStyle: path` so the
resulting URI passes `sam`'s own client-side ECR-URI-shape validation
(`--image-repositories` rejects the default `hostname`-style URI as "not a
valid ECR URI" even though it resolves and works):

```
$ aws ecr create-repository --repository-name kbdb-floci
$ REPO_URI=$(aws ecr describe-repositories --repository-names kbdb-floci \
    --query 'repositories[0].repositoryUri' --output text)
# -> localhost:5100/000000000000/us-east-2/kbdb-floci

$ sam deploy --resolve-s3 --image-repositories "ApiFunction=$REPO_URI" ...
Successfully created/updated stack - kbdb-floci-explicit in us-east-2
```

Confirmed the resulting Lambda is image-backed
(`PackageType: Image`) and the full `sam deploy` flow completes, including
the changeset and stack update — no `aws cloudformation create-stack`
workaround needed.

## Why this file stays

Left in place, retracted rather than deleted, since a wrong finding that
silently disappears is worse than one marked wrong - anyone who read the
original claim (or filed it upstream) should see the correction in the
same place.

## Consumer-side note (kbdb)

`scripts/func-setup.sh` does exactly this:
1. sets `FLOCI_SERVICES_ECR_URI_STYLE=path` on the floci container
   (`docker-compose.floci.yml`),
2. `aws ecr create-repository` explicitly, and
3. passes `--image-repositories` to `sam deploy` instead of
   `--resolve-image-repos` — mirroring `dev-deploy.sh`/CI, not inventing a
   new pattern.

That also means the `aws cloudformation create-stack`/`update-stack`
workaround adopted for floci-issues/05 can be dropped once `sam deploy`
itself works end-to-end; issue 05 still stands on its own, since it
reproduces with `sam deploy` too once more than one parameter is passed.
