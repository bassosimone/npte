# star — auditing notes

## Why this file exists

`star` is allowlisted for NOPASSWD sudo execution by `npte sudoers`,
on the same line as `netns *` and `netem *`. That allowlist is only
safe because every code path under `star/` is a pure composition of
*namespace-scoped* primitives — operations that touch private kernel
state inside namespaces the user owns, never host-namespace state.

If a future change adds a single call that reaches outside that
boundary, the sudoers contract silently breaks: a user with the
snippet installed gains passwordless ability to invoke that call
through `npte star ...`.

This file is the audit checklist for changes under `star/`.

## The invariant

Every operation `star` performs MUST be expressible as a child
invocation of one of:

- `npte netns create | destroy | connect | assign-addr | add-route | run`
- `npte netem apply | clear`

That is what `runSelf` is for, and that is the only kernel-touching
mechanism this package is allowed to use.

## What `star` MUST NOT call

The following are forbidden inside `star/`, whether via `runSelf`,
`subprocess.MustRun`, or any other path:

- `npte gateway *` — installs host-side iptables/sysctl. The whole
  reason `star` no longer calls this is to keep the NOPASSWD line
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
> `sudo npte star <new-thing> ...`, would they gain a privilege
> they did not have via `sudo npte netns ...` or `sudo npte netem
> ...` alone?"

If the answer is yes — or "maybe" — the change does not belong here.
Split it out as its own subcommand (the `gateway` shape is the
template) and let the user invoke it with a password prompt.

## Related files

- `internal/cli/sudoers/sudoers.go` — emits the NOPASSWD snippet
  that includes `npte star *`. If you change which subcommand
  classes are allowlisted, update both.
- `internal/cli/tutorial/chapters/090-sudoers.md` — user-facing
  description of the same boundary.
