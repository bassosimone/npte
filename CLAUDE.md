# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

npte (Network Performance Testing Environment) is a Go CLI tool that creates isolated Linux
network namespaces with traffic shaping to test client network performance under realistic
conditions (latency, bandwidth, packet loss). It builds a star topology with a central router
namespace and shapes traffic on the access link using `tc`/`netem`.

## Build and Test Commands

```bash
# Build
go build .

# Run tests (none currently exist)
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
```

There is no Makefile or CI configuration. The project requires Go 1.25+.

## Architecture

**Single-package design** — all Go files live in the `main` package at the repository root.

### CLI Structure

Hierarchical command dispatcher using `vclip`. Commands are registered in `main.go`:

```
npte doctor|tutorial
npte project create
npte netns create|up|down|run|show|status
npte container create|run
npte netem apply|clear
```

Each command is implemented in its own file named after the command path (e.g., `netnsup.go`,
`netemapply.go`, `projectcreate.go`).

### Key Files

- **`state.go`** — Core data structures (`netnsConfig`, `hostConfig`), config file I/O,
  validation, IP allocation. Config is stored as JSON at `/var/local/npte/<project>/config/netns.json`.
- **`environ.go`** — Side-effect abstraction layer (`environ` struct) that wraps filesystem,
  command execution, file locking, and `os.Exit`. Enables testing without root privileges.
- **`run.go`** — Command execution helpers (`runCmd`/`mustRunCmd` for shell strings,
  `runArgs`/`mustRunArgs` for arg slices). Uses `shellquote.Split` for safe parsing.
- **`log.go`** — Colored logging: `logError` (red), `logDetails` (gray), `logCommand` (blue).
- **`netem.go`** — Parses RTT duration strings and computes one-way delay for `tc netem`.

### Network Topology

Star topology stored in `/var/local/npte/<project>/`:
- Router gets /24 subnet index 0; endpoints get auto-allocated /24 subnets (indices 1+)
- All endpoints route through the router; the router NATs to the host via iptables MASQUERADE
- `netnsup` enlarges TCP buffers and loads `tcp_bbr` kernel module
- Config persists across reboots; kernel resources (namespaces, veth pairs) are ephemeral

### Embedded Documentation

Tutorial chapters live in `docs/tutorial/*.md` and are embedded into the binary via `go:embed`.
The `tutorial` command renders them with `glamour`.

## Conventions

- Commands receive `context.Context` and args; fatal errors log and call `env.Exit(1)`
- Identifiers match `^[a-z][a-z0-9]*$`; interface names are capped at 15 chars (IFNAMSIZ)
- Config access is serialized with file locks (`netns.lock`)
- The project requires root and Linux-specific tools (`ip`, `tc`, `iptables`, `sysctl`,
  `systemd-run`, `systemd-nspawn`, `debootstrap`) — run `npte doctor` to check
