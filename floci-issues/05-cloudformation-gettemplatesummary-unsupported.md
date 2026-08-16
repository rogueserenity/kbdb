# UNCONFIRMED — GetTemplateSummary failure not currently reproducible

The original version of this file reported `aws cloudformation deploy`
failing with `UnknownAction ... GetTemplateSummary` as soon as a second
`--parameter-overrides` value was passed, and proposed `create-stack` /
`update-stack` as a workaround.

## What's actually known

The failure was observed once, with this exact command:

```
aws cloudformation deploy --template-file .aws-sam/build/template.yaml \
  --stack-name kbdb-floci --parameter-overrides SkipApiRepository=true \
                          OidcIssuerBaseUrl=http://localhost.floci.io:4566 \
  --capabilities CAPABILITY_IAM --no-fail-on-empty-changeset
```

```
aws: [ERROR]: An error occurred (UnknownAction) when calling the
GetTemplateSummary operation: Action GetTemplateSummary is not supported.
```

Re-running the **identical** command against a freshly restarted floci,
three times in a row, succeeded every time - no `GetTemplateSummary` call
appears in floci's logs for any of those runs. `grep -rc GetTemplateSummary`
against floci's CloudFormation service source still returns no matches, so
if the action really is called under some condition, it would still fail -
but nothing found so far reproduces that condition.

Also retested via `sam deploy` (not just `aws cloudformation deploy`) with
two parameters and an explicit `--image-repositories` - succeeded cleanly,
no `GetTemplateSummary` call.

## Current guess, unverified

The AWS CLI's `deploy` command only calls `GetTemplateSummary` under
specific conditions (transform detection, or capability
auto-detection when `--capabilities` isn't explicit). The one run that
failed may have differed from every retry in a way not yet identified -
possibly a stale local `~/.aws/cli/cache` entry, possibly something about
the state of the previous stack rather than the command itself. Worth
noting the failing run was the *first* deploy attempt with the new
`OidcIssuerBaseUrl` parameter present in the template at all; every retry
since has been against a template that already had it.

## What this means for the migration

Not filing this upstream until it reproduces reliably with a known trigger.
The `create-stack`/`update-stack` workaround already written into
`scripts/func-setup-floci.sh` is harmless to keep regardless - it works,
and unlike `deploy` it doesn't create a changeset, so it doesn't need
whatever `deploy` triggers. But the claim that plain `deploy` requires it
is retracted until reproduced deliberately, not accidentally.
