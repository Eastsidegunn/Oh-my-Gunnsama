# Oh-My-Gunnsama

[![License](https://img.shields.io/badge/license-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.22+-blue.svg)](go.mod)
[![Status](https://img.shields.io/badge/status-experimental-orange.svg)]()

Host-neutral agent orchestration daemon. A Go-based runtime that aims to
absorb agent definitions from arbitrary ecosystems (OMO, LangChain, CrewAI,
AutoGen, OpenAI Assistants, …) into a single semantic model.

## Status

**Experimental scaffolding.** `v0.1.0` lays the foundational abstractions
but the orchestrator does not yet use them. See
[CHANGELOG.md](CHANGELOG.md) for what works today.

**Not accepting contributions yet** — the architecture is still moving.
File issues if you want, but expect breaking changes between every minor
release until `v1.0.0`.

## What it is

The thesis: standardless agent ecosystems have **semantically equivalent**
components (model, prompt, hook, skill, MCP, permissions, sub-agent),
just expressed differently. A host-neutral runtime can absorb all of them
if it offers the right primitive abstractions.

This repo is the runtime that holds those abstractions. The absorption
itself (translating a LangChain chain into our model, for example) will
need LLM assistance — that translation tool comes later.

## Architecture (current)

```
cmd/omg/                      CLI + daemon entrypoint
internal/
├── protocol/                 Host-neutral wire envelope (v1)
├── adapter/                  Worker interface + host adapters
├── daemon/                   Unix-socket server
├── orchestrator/             Run → WorkItem → AgentSession lifecycle
├── route/                    Trigger-based agent routing
├── registry/                 Agent registry (YAML + code-defined)
├── config/                   Multi-source config loader
├── prompt/                   Prompt section composition
├── skills/                   4-scope skill registry
├── mcp/                      Per-session MCP tool router
├── hook/                     5-slot hook composition
├── agents/                   Code-defined agents (sisyphus/hephaestus/oracle)
├── guard/                    Policy evaluation
├── verify/                   Verification plan execution
├── state/                    Lifecycle state machine
└── storage/                  Event-sourced sink (SQLite)

adapters/
├── claude/                   Claude Code host adapter
├── codex/                    OpenAI Codex host adapter
├── opencode/                 opencode host adapter
└── pi/                       Pi worker (LLM executor)
```

## Build

Requires Go 1.22 or newer.

```bash
git clone git@github.com:Eastsidegunn/Oh-my-Gunnsama.git
cd Oh-my-Gunnsama
go build -o omg ./cmd/omg/
```

## Run

The binary speaks a JSON protocol on stdin/stdout or via Unix socket.

```bash
# Direct invocation (no daemon) — needs pi worker on PATH
./omg run --goal "say hello"

# Help
./omg --help
```

Without the `pi` worker installed you will see a `worker factory` error.
That is expected — the runtime works, the executor is just absent.

## Test

```bash
go test -race -count=1 ./...
go vet ./...
gofmt -l .
```

All 21 packages pass with race detection on `v0.1.0`.

## Working directory

`.scratch/` is the personal working directory and is gitignored. Drop
notes, experiments, machine-specific configs there freely.

`.omx/` and `.omg/` are gitignored too — they hold local working memory,
plans, and runtime state.

## Roadmap

- `v0.1.x` — abstractions verified in isolation (current)
- `v0.2.x` — plug-in seams in the orchestrator so abstractions actually wire in
- `v0.3.x` — six universal primitives (Flow / Memory / Schema / Template / Conversation / Decision)
- `v0.4.x` — LLM-assisted absorption tool (LangChain → us, CrewAI → us, …)
- `v1.0.0` — stable API, contributions welcome

## License

Apache License 2.0. See [LICENSE](LICENSE).
