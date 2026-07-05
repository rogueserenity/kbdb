# kbdb

kbdb is a keyboard collection database (keyboards, switches, keycap sets, assembled builds), migrating from a set of linked Notion databases. The primary interface is an MCP server for AI chat clients, with REST as a secondary interface — both share the same service/repository layer.

The project is being built issue-by-issue against a fixed architecture. It's currently in Phase 0: the scaffolding (auth, routing, MCP layer, CI/CD, local dev loop) is in place; the real keyboard/switch/keycap/build data model comes in Phase 1.

## Stack

Go, AWS Lambda (container image), API Gateway, Cognito, DynamoDB (Phase 1), AWS SAM. [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) for the MCP layer.

## Getting started

Tool versions are pinned via [mise](https://mise.jdx.dev/):

```sh
mise install
```

Common tasks (see `mise.toml` for the full list):

```sh
mise run lint          # golangci-lint + actionlint + shellcheck
mise run test          # unit tests
mise run func-setup    # bring up a local dev loop (LocalStack + mockoidc + sam local start-api)
mise run func-test     # run functional tests against it
mise run func-teardown # tear it down
```

Deploying to AWS uses per-developer stacks (`mise run dev-setup`/`dev-deploy`/`dev-teardown`) rather than one shared environment — see `CLAUDE.md` for the full account/deploy model.

## Documentation

`CLAUDE.md` is the primary technical reference: architecture decisions, package layout, testing strategy, AWS account structure, and CI/CD design, along with the reasoning behind each. It's written for an AI coding agent but is equally useful as a human-readable architecture doc.

GitHub issues in this repo are the authoritative source of what's built, in progress, or deferred.

## License

Private project, not yet licensed for reuse.
