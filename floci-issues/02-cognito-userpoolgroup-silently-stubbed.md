# AWS::Cognito::UserPoolGroup reports CREATE_COMPLETE but creates nothing

**Severity:** high — silent data-plane divergence, not a visible failure
**Component:** `services/cloudformation/CloudFormationResourceProvisioner`
**Found against:** `rogueserenity/floci` @ `fix/sam-httpapi-transform`

## Symptom

A template declaring a Cognito user pool group deploys "successfully", and
CloudFormation reports the resource created:

```
$ aws --endpoint-url http://localhost:4566 cloudformation describe-stack-resources \
    --stack-name kbdb-floci --query "StackResources[?LogicalResourceId=='AdminsGroup']"
AdminsGroup   AWS::Cognito::UserPoolGroup   CREATE_COMPLETE
```

The group does not exist:

```
$ aws --endpoint-url http://localhost:4566 cognito-idp list-groups \
    --user-pool-id us-east-2_874a40c50
{
    "Groups": []
}
```

## Root cause

`AWS::Cognito::UserPoolGroup` is not in the provisioner's resource-type
switch, so it hits the default branch:

```java
default -> {
    if (resourceType != null && resourceType.startsWith("Custom::")) {
        provisionCustomResource(...);
    } else {
        LOG.debugv("Stubbing unsupported resource type: {0} ({1})", resourceType, logicalId);
        resource.setPhysicalId(logicalId + "-" + UUID.randomUUID().toString().substring(0, 8));
        resource.getAttributes().put("Arn", "arn:aws:stub:::" + logicalId);
    }
}
...
resource.setStatus("CREATE_COMPLETE");
```

The stub assigns a plausible physical id and ARN and marks the resource
`CREATE_COMPLETE`, with only a `debug`-level log line to indicate that
nothing happened.

## Why this is worse than an error

The data plane already supports everything needed — `CreateGroup`,
`AdminAddUserToGroup`, and emitting the `cognito:groups` claim are all
implemented in `CognitoService`. Only the CloudFormation binding is
missing.

So the failure surfaces far from its cause: a consumer adds a user to a
group that silently doesn't exist, gets a token with no `cognito:groups`
claim, and sees authorization failures (403s) in application code. The
natural first suspicion is a bug in the application's own authz logic, not
a CloudFormation resource that reported success.

For kbdb this manifests as three functional specs failing with 403 —
`RequireAdmin` rejects the admin test user because the claim is absent.

## Suggested fix

Add `AWS::Cognito::UserPoolGroup` to the provisioner, delegating to the
already-present `CognitoService.createGroup`. Properties needed are
`GroupName`, `UserPoolId`, and optionally `Description`, `Precedence`,
`RoleArn`.

Independently worth considering: raise the stub log from `debug` to `warn`.
A resource silently reporting `CREATE_COMPLETE` while doing nothing is the
kind of thing an operator wants to see by default, and it would have made
this diagnosable in seconds rather than by inspection.

## Scope note

`AWS::Cognito::UserPoolDomain` is stubbed identically. It has no data-plane
consequence for kbdb (hosted-UI only), so it is not filed separately —
but if `UserPoolGroup` is added, `UserPoolDomain` is the natural companion.
