# lab — auditing notes

## Why this file exists

`lab` is allowlisted for NOPASSWD sudo execution by `npte sudoers`,
on the same line as `netns *` and `netem *`. That allowlist is only
safe because every code path under `lab/` is a pure composition of
*namespace-scoped* primitives — operations that touch private kernel
state inside namespaces the user owns, never host-namespace state.

If a future change adds a single call that reaches outside that
boundary, the sudoers contract silently breaks: a user with the
snippet installed gains passwordless ability to invoke that call
through `npte lab ...`.

This file is the audit checklist for changes under `lab/`.

## The invariant

Every operation `lab` performs MUST be expressible as a child
invocation of one of:

- `npte netns create | destroy | connect | assign-addr | add-route | run`
- `npte netem apply | clear`

That is what `runSelf` is for, and that is the only kernel-touching
mechanism this package is allowed to use.

## What `lab` MUST NOT call

The following are forbidden inside `lab/`, whether via `runSelf`,
`subprocess.MustRun`, or any other path:

- `npte gateway *` — installs host-side iptables/sysctl. The whole
  reason `lab` no longer calls this is to keep the NOPASSWD line
  defensible.
- `npte container *` — runs `systemd-nspawn` with capabilities that
  reach beyond the namespace.
- Direct `iptables` / `nft` / `sysctl` / `ip link` / `tc` calls. If
  you need any of these, they belong inside an existing primitive
  (or a new one), not inlined here.
- Anything that writes outside `/run/netns/<ns>` or
  `/etc/netns/<ns>/`.

## The audit test

When reviewing a change to this package, ask:

> "If a user installed the `npte sudoers` snippet and then ran
> `sudo npte lab <new-thing> ...`, would they gain a privilege
> they did not have via `sudo npte netns ...` or `sudo npte netem
> ...` alone?"

If the answer is yes — or "maybe" — the change does not belong here.
Split it out as its own subcommand (the `gateway` shape is the
template) and let the user invoke it with a password prompt.

## Topology naming is load-bearing for the MCP

The namespace names (`client`, `router`, `server`) and the veth
interface names (`if-client`, `if-server` on the router side;
`if-router` on each leaf) are quoted verbatim in the "Lab shaping
convention" paragraph of the MCP server's handshake `instructions`
block (`internal/cli/mcp/serve.go`). That paragraph teaches the
agent where to install netem (always at the router, never at a
leaf) and refers to specific `<ns> <iface>` pairs.

If you rename any namespace or interface, change the leaf/hub
count, or add a fourth node, **update the MCP instructions
paragraph in the same change.** Drifting names there is silent:
the agent will follow stale guidance and install qdiscs on
interfaces that no longer exist (failure) or, worse, on
correctly-named interfaces in the wrong topological role (silently
wrong results).

## Related files

- `internal/cli/sudoers/sudoers.go` — emits the NOPASSWD snippet
  that includes `npte lab *`. If you change which subcommand
  classes are allowlisted, update both.
- `internal/cli/tutorial/chapters/090-sudoers.md` — user-facing
  description of the same boundary.
- `internal/cli/mcp/serve.go` — handshake `instructions` block
  quotes this package's namespace and interface names; see the
  "Topology naming is load-bearing for the MCP" section above.
