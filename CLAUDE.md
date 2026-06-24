# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

npte (Network Performance Testing Environment) is a Go CLI tool for testing how a network
client behaves on a realistic access link. It creates isolated Linux network namespaces,
wires them together with veth pairs, and shapes the links with `tc`/`netem` — no real
hardware or second machine required.

**Primary goal**: support the development of clients that maximize single-client throughput
and minimize per-client CPU usage.

## Build and Test Commands

```bash
# Build
go build .

# Run tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
```

There is no Makefile or CI configuration. The project requires Go 1.25+.

## Architecture

npte is a collection of small, composable primitives rather than a single orchestrator.
The root `main` package is a single `main.go` that wires a `vclip` dispatcher; each
subcommand lives in its own package under `internal/cli/`.

### CLI structure

```
npte doctor                               — check that required host tools are installed
npte tutorial [chapter|all]               — render embedded tutorial chapters

npte netns   create|destroy|connect|      — primitive kernel operations on one namespace
             assign-addr|add-route|run      at a time; no persisted state

npte gateway create|destroy               — turn a namespace into an internet gateway
                                            (uplink veth + host MASQUERADE/FORWARD)

npte gencerts                              — generate self-signed TLS certificates
                                            for testing (cert.pem + key.pem)

npte netem   apply|clear                  — thin wrapper around `tc qdisc ... netem`
                                            on a single <ns> <if>; supports an optional
                                            child qdisc at parent 1: for AQM experiments

npte container create|run|boot            — debootstrap + systemd-nspawn, with an
                                            optional --netns binding

npte lab     create|destroy|netem         — composes the `netns` primitives
                                            into a fixed client-router-server
                                            topology. Does NOT call `gateway`
                                            or `container`; layer those on
                                            top when needed.

npte mcp     serve                        — stdio MCP server exposing the
                                            `netns`/`netem`/`lab` primitives
                                            as tools to a coding agent that
                                            cannot shell to sudo. Forks
                                            `sudo npte ...` as a child;
                                            inherits the sudoers and
                                            `--sandbox` bounds.
```

Every leaf subcommand that touches the kernel supports `--dry-run`, which prints a
round-trippable shell script to stdout instead of executing.

### Key internal packages

- **`internal/cli/<name>/`** — one package per top-level subcommand, each exporting
  `Main(ctx, args)`. A package with subcommands builds its own nested `vclip` dispatcher
  (see `container/container.go`, `netem/netem.go`, etc.).
- **`internal/testable/`** — `Environ` struct abstracts side effects (filesystem, exec,
  file locking, `os.Exit`, stdio, log renderer). `testable.Env` is the production
  instance; tests swap in their own `Environ` to run without root or real I/O.
- **`internal/subprocess/`** — `MustRun` / `MustRunTolerant` execute a command, or print
  its round-trippable shell form when `dryRun` is true. `pipeline.go` handles piped
  invocations (e.g. `iptables-save | grep | iptables-restore`).
- **`internal/validate/`** — syntactic validators for user-supplied identifiers:
  `NetnsName`, `IfaceName`, `Username`, `DebootstrapSuite`, `CIDR`, etc.
- **`internal/logx/`** — colored logging: `Error` (red), `Details` (gray), `Command`
  (blue). Uses lipgloss via `testable.Env.LogRenderer`.
- **`internal/deps/`** — declares the external binaries npte is allowed to exec;
  `npte doctor` reads this list.

### Shape of a subcommand

Each subcommand file follows the same shape:

1. Build a `vflag.FlagSet`, wire `Exit`/`Stderr`/`Stdout`/`UsagePrinter` from
   `testable.Env`, parse `args`.
2. Validate positional arguments with `internal/validate`.
3. Log a one-line `logx.Details` explaining what is about to happen.
4. Call `subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ...)` (or similar)
   to do the kernel work.

No command persists state; topologies are built imperatively by composing primitives.
`lab` is the one exception — it hard-codes a specific composition.

### Embedded documentation

Tutorial chapters live in `internal/cli/tutorial/chapters/NNN-*.md` and are embedded
via `go:embed`. The leading `NNN-` prefix orders them; `tutorial.Main` strips the
prefix, extracts the `# Title`, and auto-generates the TOC. Chapter slugs are stable
(`netns-basics`, `routing`, `netem`, `bufferbloat`, `browser`, `containers`, `podman`,
`sudoers`, `sandbox`, `mcp`, `gencerts`).

## Conventions

- Leaf subcommands receive `context.Context` and `args []string`; fatal errors log and
  call `env.Exit(1)` or `env.Exit(2)` (2 = usage error, 1 = runtime error).
- User-supplied identifiers are validated with `internal/validate`; interface names
  are capped at 15 chars (IFNAMSIZ).
- Per user preference: `len(x) <= 0` rather than `== 0` for emptiness checks.
- The project requires root and Linux-specific tools (`ip`, `tc`, `iptables`, `sysctl`,
  `systemd-nspawn`, `debootstrap`, `runuser`). Run `npte doctor` to check.
