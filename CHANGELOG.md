# Changelog

All notable changes to Oh-My-Gunnsama are documented in this file.

The format roughly follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project does not yet follow semantic versioning strictly — minor versions
may break API until `v1.0.0`.

## [Unreleased]

### Pending
- Plug-in seams in the orchestrator (so the v0.1.0 abstractions actually wire in)
- Six universal primitives: Flow, Memory, Schema, Template, Conversation, Decision

## [0.1.0] — 2026-05-21

First tagged release. Establishes the foundational abstractions for
agent ecosystem absorption.

### Added
- `internal/hook/` — 5-slot composition (input / output / guard / lifecycle / flow)
- `internal/skills/` — 4-scope skill registry (project > opencode > user > builtin)
- `internal/mcp/` — Per-session tool router with isolation
- `internal/prompt/` — Dynamic `Section` interface for prompt assembly
- `internal/agents/` — Code-defined agents (sisyphus, hephaestus, oracle)
- `internal/registry/code.go` — Programmatic `Register()` API (additive to YAML loader)
- `internal/adapter/mockworker.go` — Worker test double
- `internal/storage/memory.go` — In-memory `Sink` for tests
- Untracked `.scratch/` working directory pattern

### Verified
- 80+ tests across 21 packages, race-clean
- `TestNonInvasiveGap_*` tests document that the core orchestrator does not
  yet consume the new abstractions (intentional — proves abstractions stand
  on their own without invasive wiring).

### Known limitations
- The orchestrator does not call the new hook/skill/mcp/prompt code.
- Code-registered agents are not visible through `registry.Build()`.
- These gaps are intentional and will close in `v0.2.x` via plug-in seams.

### Not in scope
- LangChain / CrewAI / AutoGen absorption (planned for `v0.4.x` via LLM tool)
- Stateful conversation primitives
- Output schema validation

[Unreleased]: https://github.com/Eastsidegunn/Oh-my-Gunnsama/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Eastsidegunn/Oh-my-Gunnsama/releases/tag/v0.1.0
