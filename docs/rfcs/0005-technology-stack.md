# RFC 0005: Technology Stack Decision

Status: Draft
Date: 2026-04-30
Target release: Nomici Orchestrator v0.1

## Summary

Nomici Orchestrator should use a hybrid stack:

- Go for the CLI, Gateway, runtime manager, adapters, policy, trace store, and release binary.
- TypeScript, React, Vite, React Flow, Tailwind, and pnpm for Nomici Console and JavaScript packages.
- Makefile as the contributor-facing command layer.
- A single `nomici` binary as the end-user-facing command layer.

This keeps the product install and runtime experience simple while still using the right ecosystem for a modern visual canvas.

## Decision

The release artifact for v0.1 is a Go binary:

```bash
nomici
```

End users should not need Node.js, pnpm, Vite, or Go installed to run Nomici after installation.

Contributors may need Go, Node.js, and pnpm, but they should interact with the repository through stable root commands:

```bash
make dev
make build
make test
make lint
make fmt
make clean
```

Internal implementation can call Go and pnpm commands, but documentation, CI, and contributor onboarding should prefer the Makefile commands.

## Rationale

Nomici has two different engineering surfaces.

The control-plane surface needs:

- Single binary distribution.
- Cross-platform CLI behavior.
- Gateway daemon support.
- Local process management.
- Log streaming.
- HTTP and WebSocket APIs.
- SQLite persistence.
- Safe secret handling.
- Low operational dependency burden.

Go is the better default for this surface.

The Web Console surface needs:

- A rich browser UI.
- Canvas and graph editing.
- Fast frontend iteration.
- Strong component ecosystem.
- Type-safe API clients.
- A maintainable open-source contributor path for UI work.

TypeScript and React are the better default for this surface.

The important product constraint is not "one programming language." The important product constraint is "one user command."

## End-User Experience

End users install and run:

```bash
curl -fsSL https://nomici.ai/install.sh | bash
nomici init --template ai-application-pm
nomici up
nomici gateway open
```

They should not run:

```bash
pnpm install
pnpm build
go build
```

The release binary must embed the built Web Console assets. The Gateway serves those assets directly.

## Contributor Experience

Contributors use root-level commands:

```bash
make dev
make build
make test
make lint
make fmt
make clean
```

Expected behavior:

```text
make dev
  Start the Go Gateway in development mode.
  Start the Vite dev server for Nomici Console.
  Configure the Console to talk to the local Gateway.

make build
  Install frontend dependencies if needed.
  Build Nomici Console static assets.
  Embed or copy the Console assets for Go embedding.
  Build the nomici binary.

make test
  Run Go tests.
  Run TypeScript tests.
  Run schema validation tests.

make lint
  Run Go vet/lint checks.
  Run TypeScript lint checks.
  Validate generated files are current.

make fmt
  Run gofmt.
  Run Prettier.
```

The Makefile is the stable contributor interface. It may delegate internally to scripts as the repository grows.

## Build Boundary

The build pipeline should look like this:

```text
apps/web source
  -> pnpm --filter @nomici/web build
  -> apps/web/dist
  -> embedded into Go Gateway with go:embed
  -> go build ./cmd/nomici
  -> bin/nomici
```

The runtime pipeline should look like this:

```text
nomici gateway run
  -> starts Go Gateway
  -> serves embedded Console assets
  -> exposes REST/WebSocket APIs
  -> manages local runtimes
  -> stores local state in SQLite
```

No Node.js process should be required in production mode.

## Repository Layout

Recommended layout:

```text
orchestrator/
  Makefile
  go.mod
  go.sum
  package.json
  pnpm-lock.yaml
  pnpm-workspace.yaml

  cmd/
    nomici/

  internal/
    gateway/
      web/
        dist/              # generated or copied frontend assets for embedding
    runtime/
    registry/
    policy/
    trace/
    secrets/
    spec/

  apps/
    web/
      package.json
      index.html
      src/

  packages/
    spec/
      nomici.schema.json
      package.json
    client-js/
      package.json

  adapters/
    hermes/
    openclaw/

  examples/
  scripts/
  docs/
```

Generated files must be clearly marked. If `internal/gateway/web/dist` is generated, CI should verify it is current for release builds.

## Go Stack

Recommended:

- Go 1.23 or newer, subject to current stable release at implementation time.
- `spf13/cobra` for CLI command structure.
- `net/http` plus `go-chi/chi` for Gateway routing.
- SQLite through a driver that supports cross-platform release builds.
- `go:embed` for Web Console assets.
- Standard library logging initially, with structured logging abstraction added when trace needs demand it.

Initial Go modules:

- `cmd/nomici`: CLI entrypoint.
- `internal/gateway`: HTTP server and embedded Console.
- `internal/spec`: AgentSpec loading and validation.
- `internal/runtime`: local process runner.
- `internal/trace`: SQLite event store.
- `internal/policy`: policy and approval decisions.
- `internal/registry`: normalized model, agent, runtime, and tool registries.

Avoid large Go frameworks in v0.1. The Gateway is a control-plane API server, not a general web application framework.

## TypeScript Stack

Recommended:

- TypeScript.
- React.
- Vite.
- `@xyflow/react` for the canvas.
- Tailwind for styling.
- pnpm workspaces.

Initial packages:

- `apps/web`: Nomici Console.
- `packages/spec`: AgentSpec JSON Schema and generated TypeScript types.
- `packages/client-js`: future JavaScript client for Gateway API.

The Web Console should call Gateway APIs rather than duplicate control-plane logic in TypeScript.

## Spec and Type Generation

AgentSpec should use JSON Schema as the contract format.

Early v0.1 can maintain Go structs and JSON Schema manually if generation overhead slows iteration. Before v0.1 release, the project should either:

- generate TypeScript types from JSON Schema, or
- validate that manually maintained TypeScript types match schema.

The source of truth for validation behavior is the Gateway's `internal/spec` package and the published JSON Schema.

## Adapter Strategy

v0.1 adapters should be implemented in Go when they are simple HTTP adapters:

- OpenAI-compatible endpoint adapter
- Hermes endpoint adapter
- OpenClaw endpoint adapter
- Ollama or vLLM provider adapter

Future deeper adapters may use sidecars:

- Python sidecar for LangGraph and OpenAI Agents SDK.
- TypeScript sidecar for Node-native agent frameworks.
- A2A sidecar for runtimes that do not expose A2A directly.

The stack decision should not block future Python or TypeScript adapters. It only says the local control plane and release binary are Go-first.

## CI Expectations

CI should use the same commands contributors use:

```bash
make fmt
make lint
make test
make build
```

CI should also verify:

- Go binary builds on Linux, macOS, and Windows.
- Web Console builds with pinned pnpm lockfile.
- Embedded asset build is reproducible enough for release.
- AgentSpec schema is valid.
- Generated code is current.

## Release Expectations

Release artifacts:

- `nomici` binary for macOS, Linux, and Windows.
- Checksums.
- SBOM.
- Docker image later.
- Homebrew tap later.
- npm package later if needed for JS client or thin installer.
- PyPI package later if needed for Python adapters.

The npm package must not become the primary way to run the control plane unless a future RFC explicitly changes this decision.

## Tradeoffs

Benefits:

- End users get a simple single-binary experience.
- The Gateway can manage processes and local state without Node.js.
- The Web Console can use the best available graph UI ecosystem.
- Contributors get a clear command surface through `make`.
- The project can still publish JS and Python SDKs later.

Costs:

- Contributors need both Go and Node.js for full-stack work.
- CI must cache both Go modules and pnpm dependencies.
- Type definitions must be kept aligned across Go, JSON Schema, and TypeScript.
- Build orchestration needs discipline so `make build` remains reliable.

## Rejected Alternatives

### All TypeScript

Rejected for v0.1 because it weakens the single-binary local control-plane story and makes production Gateway/runtime management depend on Node.js.

TypeScript remains the right choice for Console and JS packages.

### All Go

Rejected because the Web Console needs a rich graph/canvas ecosystem and fast UI iteration. Rebuilding that in Go, server-rendered HTML, or WebAssembly would slow the product down and reduce contributor accessibility.

### Rust Control Plane

Rust is a strong fit for a single binary and systems code, but Go is simpler for the expected contributor base, local process management, HTTP APIs, and rapid control-plane iteration.

### Python Control Plane

Python is excellent for agent-framework adapters, but it is not ideal as the primary local Gateway and CLI distribution target for this project.

## Open Questions

- Should `make dev` run Vite separately or serve only embedded assets in early development?
- Should release builds commit generated `apps/web/dist` or produce it only in CI?
- Which SQLite driver should be selected for the first implementation?
- Should the project use `Cobra` from the start or begin with standard `flag` and migrate later? Recommendation: start with Cobra because the CLI surface is known to be broad.
- Should generated TypeScript types live in `packages/spec` from day one? Recommendation: yes, even if generation is added after the first scaffold.
