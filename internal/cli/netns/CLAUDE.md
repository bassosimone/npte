# NOPASSWD audit invariant — `internal/cli/netns`

The subcommands in this package (`npte netns *`) are part of the set
that `npte sudoers` allowlists for **NOPASSWD** sudo execution. After
a user installs the snippet emitted by `npte sudoers`, anyone with
write access to that user's account can invoke any command in this
package via sudo without authenticating.

That changes the threat model for code in this directory:

- The **user** is trusted: sudo authorized them.
- The **bytes** the user supplies — flag values, positionals,
  environment values — are NOT trusted. They may be attacker-
  controlled if the user's shell, scripts, or some upstream pipeline
  are.
- A missing validator on a value that reaches a subprocess argv is a
  passwordless privilege-escalation hole.

## When editing this package

1. Every flag value, positional argument, or environment value
   forwarded to a subprocess must be passed through a validator
   from `internal/validate`, OR be a hardcoded literal.
2. When adding a flag, positional, or new subcommand, audit the
   full path from `vflag.Parse` to `subprocess.MustRun`. Each
   intermediate step should be entitled to assume: "if I got here,
   the input is well-formed and bounded."
3. New validators belong in `internal/validate`, with tests
   covering at least: empty input, oversized input, shell
   metacharacters, leading hyphens (argv option confusion), and
   path traversal where applicable.
4. Prefer hardcoded argv literals to user-controlled bytes wherever
   possible.

# Registry invariant — `internal/cli/netns`

The NOPASSWD grant lets a caller invoke every verb in this package as
root without authenticating. The validation invariant above bounds the
*bytes* that reach argv; the **registry invariant** bounds the
*namespaces* the privileged surface can touch.

A netns is "managed by npte" when an empty marker file exists at
`/run/npte/netns/<name>`. Markers are written by `netns create` and
removed by `netns destroy`; every other verb in this package refuses
to operate on a netns that has no marker. Without this bound the
privileged surface would include every netns on the host, not just
the ones npte itself created.

## When editing this package (registry side)

1. Every state-touching verb MUST take the global registry lock at
   the top of its run, **after shape validators** and **before any
   kernel or marker operation**:

   ```go
   unlock := registry.MustLock(ctx, env, dryRun)
   defer unlock()
   ```

   The lock serializes all npte invocations so that the
   "kernel op then marker op" sequence is observable as atomic to
   other npte processes. `MustLock` also ensures `/run/npte/netns/`
   exists with the right perms (via `install -d`) and is dry-run-aware.

2. Verbs that touch a *named* netns — i.e. everything except
   `create` — MUST call `registry.RequireManaged(env, ns)` after the
   lock and before any kernel op. A missing check means the verb can
   be exercised against a netns npte does not own — the same
   passwordless-privesc shape as a missing argv validator.

3. `netns create` MUST end with `registry.MustRegister(ctx, dryRun, ns)`
   **after** the kernel ops succeed. `netns destroy` MUST end with
   `registry.MustUnregister(ctx, dryRun, ns)` **after**
   `ip netns del` succeeds. Kernel op first, marker op second:
   orphan markers are recoverable by the operator (one-liner:
   `sudo rm /run/npte/netns/<name>`); orphan namespaces are not.
   We deliberately do not ship a recovery verb: a `gc` or a
   destroy-tolerates-missing shape would have to disambiguate
   "stale orphan marker" from "stale marker plus a same-named
   foreign netns created out-of-band," and there is no metadata
   on the marker that links it to a specific kernel-side identity,
   so any automated reconciler risks silently touching a foreign
   namespace. The crash window (between two consecutive
   subprocesses) is narrow enough in practice that operator-hands
   recovery is the right level of investment.

4. Verbs that take more than one named netns (e.g. `connect`) MUST
   call `RequireManaged` on **every** one of them. Generalizes to
   any future verb that takes N named netns.

5. Do **not** add a `--foreign` / `--force` or similar escape hatch that
   bypasses the ownership check. The two-track design (managed-only by
   default, foreign-allowed via flag) was rejected: every verb in
   this package already exposes the full kernel surface to anyone with the
   NOPASSWD grant once they pass `--foreign`. If foreign access is genuinely
   needed for debugging, `ip netns ...` directly is right there.

## References

- `internal/cli/sudoers/sudoers.go` — what is allowlisted, and the
  install-time guidance that surfaces this invariant to operators.
- `internal/validate/validate.go` — the validators this package
  relies on (`NetnsName`, `IfaceName`, `IPAddr`, `CIDR`, `Username`,
  `EnvVarName`).
- `internal/registry/registry.go` — the marker-file scheme, lock
  primitive, ownership predicate, and the kernel-op-then-marker-op
  ordering rationale.
- The comment block before the validation section in each command
  file in this package — local manifestation of the invariant.
