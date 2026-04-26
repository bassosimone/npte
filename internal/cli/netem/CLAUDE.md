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
2. `tc` accepts pass-through grammars (e.g. netem knob values,
   child qdisc args) that are tokenized via `shellquote.Split` and
   pasted into argv. These tokens must still be vetted: see
   `validate.ChildQdiscKind` (allowlist of qdisc kinds, since `tc`
   autoloads `sch_<kind>` modules) and `validate.NetemNoKnobSmuggling`
   (rejects values that name another netem knob, so each `--<flag>`
   stays scoped to its own knob).
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

## References

- `internal/cli/sudoers/sudoers.go` — what is allowlisted, and the
  install-time guidance that surfaces this invariant to operators.
- `internal/validate/validate.go` — the validators this package
  relies on (`NetnsName`, `IfaceName`, `ChildQdiscKind`,
  `NetemNoKnobSmuggling`).
- The comment block before the validation section in each command
  file in this package — local manifestation of the invariant.
