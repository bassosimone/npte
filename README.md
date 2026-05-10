# Network Performance Testing Environment

[![Build Status](https://github.com/bassosimone/npte/actions/workflows/go.yml/badge.svg)](https://github.com/bassosimone/npte/actions) [![codecov](https://codecov.io/gh/bassosimone/npte/branch/main/graph/badge.svg)](https://codecov.io/gh/bassosimone/npte)

`npte` (Network Performance Testing Environment) tests how a network client
behaves on a realistic access link, using isolated network namespaces, traffic
shaping, and optional lightweight containers.

It is a collection of small, composable primitives: create and connect
namespaces (`netns`), attach a host-NATed uplink (`gateway`), shape a link
with `tc`/`netem` (`netem`), and optionally run commands inside a
`systemd-nspawn` container (`container`). The `lab` command wires a fixed
topology for the common case.

> **Linux-only.** Most subcommands require `root` and use external commands
> (`ip`, `tc`, `iptables`, `sysctl`, `systemd-nspawn`, `debootstrap`,
> `runuser`). Run `npte doctor` to check for dependencies.

## Install

You need Go >= 1.25.

```bash
go install -v github.com/bassosimone/npte@latest
sudo install -m555 "$(go env GOPATH)/bin/npte" /usr/local/sbin/npte
rm "$(go env GOPATH)/bin/npte"
```

We prefer not installing at `$(go env GOPATH)/bin/npte` because that path
is owned and writable by the invoking user: a tool that `root` runs via
`sudo` should not live somewhere an unprivileged process can replace it,
and the absolute path baked into the `npte sudoers` snippet has to be one
that only `root` can modify.

For local development, `go build .` is fine; the resulting
binary will report its version as `(devel)`.

## Quick Start

```bash
npte doctor       # check required host tools are installed
npte tutorial     # render the embedded walkthrough
npte --help       # interactive help
```

## Tutorial

Run `npte tutorial` to see the full table of contents and read the
embedded chapters. They are also browsable on GitHub at
[`internal/cli/tutorial/chapters/`](
https://github.com/bassosimone/npte/tree/main/internal/cli/tutorial/chapters).

## Subcommands

- `doctor` — check required host tools are installed.

- `tutorial` — render the embedded tutorial chapters.

- `netns` — primitive operations on a single network namespace.

- `gateway` — turn a namespace into a host-NAT'd internet gateway.

- `netem` — apply or clear traffic shaping on a single `<ns> <if>`.

- `container` — `debootstrap` + `systemd-nspawn` helpers, with optional
  binding to an existing namespace.

- `lab` — compose a fixed three-node client/router/server topology
  out of the `netns` primitives.

- `sudoers` — print `sudoers` `NOPASSWD` configuration for selected subcommands.

All the subcommands that modify the kernel state also support the `--dry-run`
flag, which prints a round-trippable shell script to stdout instead of executing.

## License

```
SPDX-License-Identifier: GPL-3.0-or-later
```

## Direct Dependencies

- [github.com/bassosimone/closepool](https://pkg.go.dev/github.com/bassosimone/closepool)
- [github.com/bassosimone/runtimex](https://pkg.go.dev/github.com/bassosimone/runtimex)
- [github.com/bassosimone/textwrap](https://pkg.go.dev/github.com/bassosimone/textwrap)
- [github.com/bassosimone/vclip](https://pkg.go.dev/github.com/bassosimone/vclip)
- [github.com/bassosimone/vflag](https://pkg.go.dev/github.com/bassosimone/vflag)
- [github.com/charmbracelet/glamour](https://pkg.go.dev/github.com/charmbracelet/glamour)
- [github.com/charmbracelet/lipgloss](https://pkg.go.dev/github.com/charmbracelet/lipgloss)
- [github.com/kballard/go-shellquote](https://pkg.go.dev/github.com/kballard/go-shellquote)
- [github.com/rogpeppe/go-internal](https://pkg.go.dev/github.com/rogpeppe/go-internal)
- [github.com/stretchr/testify](https://pkg.go.dev/github.com/stretchr/testify)

## History

`npte` implements the strategy described in [ooni/probe#1803](
https://github.com/ooni/probe/issues/1803#issuecomment-942761976).
