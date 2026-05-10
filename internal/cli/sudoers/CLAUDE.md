# Allowlist invariant — `internal/cli/sudoers`

This package emits the `sudoers` snippet that grants **NOPASSWD**
sudo for a fixed set of npte verb globs (currently `netns *`,
`netem *`, `lab *`). Everything in those globs is invocable as
root by anyone with write access to the allowlisted user's account,
without authenticating.

The registry constraint — every verb refuses to operate on a netns
that npte did not create, enforced via `registry.RequireManaged` —
is the **bound on this NOPASSWD surface**, not a correctness check.
It exists specifically so an attacker who can write a script as the
allowlisted user cannot pivot the privileged surface against
arbitrary system namespaces (Docker, libvirt, host root netns,
etc.).

## When editing this package

1. **Adding a new verb glob to the allowlist is an audit moment.**
   Before extending the snippet (e.g. `gateway *`, `foo *`), audit
   every command in that package:
   - Does each command call `registry.MustLock(ctx, env, dryRun)`
     at the top of its run, after shape validators and before any
     kernel op?
   - Does each command call `registry.RequireManaged(env, ns)` on
     **every** named netns it accepts (positionals or flags), after
     the lock and before any kernel op?
   - Are all flag values, positionals, and environment values
     forwarded to subprocesses passed through validators in
     `internal/validate`, or hardcoded literals?

   If any answer is "no", do not add the glob until the verbs are
   brought into compliance. The package-local `CLAUDE.md` files
   (`internal/cli/netns/CLAUDE.md`, `internal/cli/netem/CLAUDE.md`)
   spell out the contract.

2. **Password-required verbs intentionally skip `RequireManaged`.**
   Outside the allowlist (currently: `gateway`, `container`,
   anything not matched by the globs above), verbs may operate on
   foreign namespaces. The user has already cleared the typed-
   password bar — the same bar as `sudo ip netns exec ...` — so
   refusing would just be paternalism that breaks composability
   with hand-rolled or third-party namespaces.

   This split is load-bearing: it is exactly what makes the
   registry constraint tolerable on the NOPASSWD side and the tool
   useful on the password side. Do not "harmonize" by adding
   `RequireManaged` to password verbs, and do not "simplify" by
   dropping it from NOPASSWD verbs.

3. **Promoting a password verb into the allowlist is the same audit
   as #1.** A verb that has lived without `RequireManaged` will
   need it added — on every named netns — before its glob can join
   the snippet. Treat the move as a security-relevant change; do
   not bundle it with unrelated work.

## References

- `internal/cli/netns/CLAUDE.md` — full NOPASSWD + registry
  contract for the `netns` verbs.
- `internal/cli/netem/CLAUDE.md` — same contract for `netem`,
  plus the netem-specific argv-vetting notes (regex grammar
  validators, `--child` allowlist).
- `internal/registry/registry.go` — marker scheme, lock primitive,
  `RequireManaged`, and the kernel-op-then-marker-op ordering
  rationale.
