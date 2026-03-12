# VEF Framework Go

📖 [English](./README.md) | [简体中文](./README.zh-CN.md)

[![GitHub Release](https://img.shields.io/github/v/release/coldsmirk/vef-framework-go)](https://github.com/coldsmirk/vef-framework-go/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/coldsmirk/vef-framework-go/test.yml?branch=main)](https://github.com/coldsmirk/vef-framework-go/actions/workflows/test.yml)
[![Coverage](https://img.shields.io/codecov/c/github/coldsmirk/vef-framework-go)](https://codecov.io/gh/coldsmirk/vef-framework-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/coldsmirk/vef-framework-go.svg)](https://pkg.go.dev/github.com/coldsmirk/vef-framework-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/coldsmirk/vef-framework-go)](https://goreportcard.com/report/github.com/coldsmirk/vef-framework-go)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/coldsmirk/vef-framework-go)
[![License](https://img.shields.io/github/license/coldsmirk/vef-framework-go)](https://github.com/coldsmirk/vef-framework-go/blob/main/LICENSE)

VEF Framework Go is an opinionated Go framework for enterprise applications. It combines Uber FX for dependency injection, Fiber for HTTP handling, and Bun for data access, with built-in support for API resources, authentication, RBAC, validation, caching, events, storage, MCP, and more.

> This README is intentionally brief. Detailed tutorials and reference material are being moved to the dedicated documentation site.

> Development status: the project is still pre-1.0. Expect breaking changes while conventions and APIs continue to evolve.

## Why VEF

- Unified API model for both RPC and REST resources
- Generic CRUD building blocks for common backend workflows
- Modular bootstrapping based on Uber FX
- Built-in authentication, RBAC, rate limiting, and audit support
- Integrated infrastructure modules such as event, CQRS, cron, redis, storage, schema, monitor, MCP, and more

## Quick Start

### Requirements

- Go 1.26.0 or newer
- A supported database such as PostgreSQL, MySQL, or SQLite

### Install

```bash
go get github.com/coldsmirk/vef-framework-go
```

### Minimal app

Create `main.go`:

```go
package main

import "github.com/coldsmirk/vef-framework-go"

func main() {
	vef.Run()
}
```

Create `configs/application.toml`:

```toml
[vef.app]
name = "my-app"
port = 8080

[vef.data_source]
type = "sqlite"
path = "./my-app.db"
```

This is a minimal configuration example. Additional sections include `vef.monitor`, `vef.mcp`, and `vef.approval`.

Run the app:

```bash
go run main.go
```

VEF loads `application.toml` from `./configs`, `.`, `../configs`, or the path pointed to by `VEF_CONFIG_PATH`.

## Core Concepts

- `vef.Run(...)` starts the framework and wires the default module chain: config, database, ORM, middleware, API, security, event, CQRS, cron, redis, mold, storage, sequence, schema, monitor, MCP, and app.
- API endpoints are defined as resources with `api.NewRPCResource(...)` or `api.NewRESTResource(...)`.
- Business modules are composed with FX options, for example `vef.ProvideAPIResource(...)`, `vef.ProvideMiddleware(...)`, and `vef.ProvideMCPTools(...)`.
- CRUD-heavy modules can build on the generic helpers in `crud/` instead of writing repetitive handlers from scratch.

Typical application layout:

```text
my-app/
├── cmd/
├── configs/
└── internal/
    ├── auth/
    ├── sys/
    ├── <domain>/
    └── web/
```

## Documentation

- API reference: <https://pkg.go.dev/github.com/coldsmirk/vef-framework-go>
- Repository knowledge map: <https://deepwiki.com/coldsmirk/vef-framework-go>
- Testing conventions: [TESTING.md](./TESTING.md)

If you need step-by-step guides, architectural deep dives, or feature-specific reference, prefer the dedicated documentation site rather than expanding this README.

## Development

Common verification commands:

```bash
go test ./...
go test -race ./...
golangci-lint run
```

Release scripts are available in the repository root, but they should only be used intentionally:

```bash
./release.sh vX.Y.Z "description"
./unrelease.sh vX.Y.Z
```

## License

Licensed under the [Apache License 2.0](./LICENSE).
