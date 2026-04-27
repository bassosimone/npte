# NOPASSWD audit invariant — `internal/cli/netem`

The subcommands in this package (`npte netem *`) are part of the set
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
2. Some `tc` invocations take pass-through grammars (netem knob
   values, child qdisc args) that ultimately become argv tokens.
   These must be vetted before reaching the subprocess:
   - The five netem flag-value validators in
     `internal/validate/netem.go` (`NetemDelay`, `NetemLoss`,
     `NetemLimit`, `NetemRate`, `NetemSlot`) anchor a whole-string
     regex modeled on `man tc-netem` against each `--<flag>` value.
     Position, arity, and atom shapes (TIME, RATE, PCT, distribution
     names, ...) all ride in the regex, so a smuggled keyword like
     `loss` inside a `--delay` value or a stray `ecn` does not match
     any branch and is rejected. After the validator returns nil
     the value is split mechanically with `strings.Fields` — no atom
     contains whitespace, so `shellquote` is not needed for these
     flags. tc(8) remains the authoritative parser; if the grammar
     evolves upstream, the fix is to extend the regex.
   - `--child` is **not** a free-form pass-through. The flag takes
     a single qdisc *kind* validated by `validate.ChildQdiscKind`
     against the narrow `validate.AllowedChildQdiscs` allowlist.
     The list is small because `tc` autoloads `sch_<kind>` kernel
     modules — a side effect that escapes the network namespace
     the qdisc lives in. Per-kind knobs are surfaced as separate
     typed CLI flags (e.g. `--cake-bandwidth`, validated via
     `NetemRate`) and added on demand. Each per-kind knob is
     validated and consumed inside the corresponding `case` in
     `apply.go`'s child-dispatch `switch`, so a value whose owning
     `--child` kind is not selected is silently ignored — its
     bytes never reach argv. New kinds/knobs are added by
     extending the allowlist and the matching `case`; do not
     reintroduce a free-form pass-through.
3. When adding a flag, positional, or new subcommand, audit the
   full path from `vflag.Parse` to `subprocess.MustRun`. Each
   intermediate step should be entitled to assume: "if I got here,
   the input is well-formed and bounded."
4. New validators belong in `internal/validate`, with tests
   covering at least: empty input, oversized input, shell
   metacharacters, leading hyphens (argv option confusion), and
   path traversal where applicable.
5. Prefer hardcoded argv literals to user-controlled bytes wherever
   possible.

# Registry invariant — `internal/cli/netem`

The NOPASSWD grant lets a caller invoke every verb in this package as
root without authenticating. The validation invariant above bounds the
*bytes* that reach argv; the **registry invariant** bounds the
*namespaces* the privileged surface can touch.

netem verbs do not create or destroy network namespaces; they shape
interfaces inside an existing one. The first positional `<ns>` MUST
refer to a namespace that npte itself created — i.e. one with a
marker at `/run/npte/netns/<ns>`. See
`internal/cli/netns/CLAUDE.md` for the full registry contract
(marker scheme, kernel-op-then-marker-op ordering, no `--foreign`
escape hatch).

## When editing this package (registry side)

1. Every verb MUST take the global registry lock at the top of its
   run, **after shape validators** and **before any kernel op**:

   ```go
   unlock := registry.MustLock(ctx, env, dryRun)
   defer unlock()
   ```

   The lock serializes all npte invocations and ensures
   `/run/npte/netns/` exists with the right perms.

2. Every verb MUST call `registry.RequireManaged(env, ns)` on its
   first positional after the lock and before any kernel op. A
   missing check means the verb can be exercised against a netns
   npte does not own — the same passwordless-privesc shape as a
   missing argv validator.

3. New netem verbs follow the same pattern. If a future verb takes
   more than one named netns, check all of them.

## References

- `internal/cli/sudoers/sudoers.go` — what is allowlisted, and the
  install-time guidance that surfaces this invariant to operators.
- `internal/validate/` — the validators this package relies on:
  `NetnsName`, `IfaceName` (in `validate.go`); `ChildQdiscKind`
  and `AllowedChildQdiscs` (in `tc.go`); and the per-flag netem
  grammar validators `NetemDelay`, `NetemLoss`, `NetemLimit`,
  `NetemRate`, `NetemSlot` (in `netem.go`).
- `internal/registry/registry.go` — the marker-file scheme, lock
  primitive, ownership predicate, and the kernel-op-then-marker-op
  ordering rationale.
- `internal/cli/netns/CLAUDE.md` — the broader registry contract
  for verbs that own marker lifecycle (`create`/`destroy`).
- The comment block before the validation section in each command
  file in this package — local manifestation of the invariant.
