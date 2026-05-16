# Forwarding contract — `internal/cli/mcp`

## Why this file exists

`npte mcp serve` runs **outside** the agent's sandbox as a long-lived
stdio process. It does not execute privileged kernel ops directly:
every MCP tool handler in this package composes an `npte ...` argv
and forks `sudo -n npte ...` as a child via `sessionManager.startProc`.
The privileged side is then bounded by the `netns`/`netem`/`lab`
audit invariants (registry, validators, `--sandbox`), which the agent
**inherits** by construction.

That construction is fragile. The agent cannot read the sudoers
allowlist, cannot inspect the registry, and cannot tell whether
`--sandbox` was injected. If this package forks the wrong verb,
forwards unvalidated bytes, or silently drops `--sandbox`, the
agent's trust bridge widens — and the agent has no signal that it
did. This file is the audit checklist for changes under `mcp/`.

The shape is parallel to `internal/cli/lab/CLAUDE.md`: `lab/` must
not call outside the NOPASSWD allowlist, and `mcp/` must not expose
to the agent anything outside that same allowlist. Different sides of
the same boundary.

## Invariants

### 1. The tool surface is bounded to the sudoers allowlist

Every MCP tool registered in `serve.go` MUST fork a subcommand
that `internal/cli/sudoers` allowlists for NOPASSWD execution
(currently: `netns *`, `netem *`, `lab *`). Concretely, the argv
tail passed to `startProc` MUST begin with one of those three verbs.

`gateway` and `container` MUST NOT be exposed as MCP tools. Both are
deliberately kept out of the sudoers allowlist (see
`internal/cli/sudoers/CLAUDE.md`):

- `gateway` installs host-namespace iptables/sysctl that the registry
  bound cannot fence.
- `container` runs `systemd-nspawn` with capabilities that escape
  the namespace.

Adding either to the MCP toolbox would either (a) hang the bridge on
a `sudo` password prompt — `startProc` forks with `sudo -n`, so a
verb outside the allowlist exits non-zero immediately, but the agent
sees a confusing failure rather than a refusal; or (b) silently
elevate the agent's authority if the operator extended the allowlist
in a private snippet. Both outcomes are bad. Keep the toolbox bounded
to verbs whose privileged side is already audited by their own
package-local CLAUDE.md.

If a new sudoers-allowlisted verb is added in the future, the audit
for exposing it via MCP is its own separate decision — write a new
tool handler only after confirming the verb's package
(`internal/cli/<name>/CLAUDE.md`) holds the registry and validator
contract, and the resulting tool input schema cannot smuggle bytes
past those checks.

### 2. `netns run` MUST inject `--sandbox` unconditionally

The agent's user-supplied code path is `start_netns_run`. That tool
MUST inject `--sandbox` into the forked argv, and the argv MUST be
composed such that no field of `netnsRunInput` (the netns name, the
env map, the argv slice) can be parsed as a flag — in particular,
none of them may flip `--sandbox=false`.

Three layers cooperate:

- `NetnsRun` in `netns.go` hardcodes `--sandbox` as the second token
  of the argv tail: `[]string{"netns", "run", "--sandbox", ...}`.
- A GNU `--` separator is inserted **before the netns positional**:
  `args = append(args, "--", input.Netns)`. This is the structural
  guarantee. Without it, `input.Netns = "--sandbox=false"` is parsed
  as a flag — vflag is still in flag-parsing mode when it reaches
  the first positional, and `DisablePermute` does not engage until
  *after* a positional has been consumed — and the inner command
  runs without bubblewrap. This was a real bypass; the `--` is the
  fix. See invariant #3 for the package-wide rule.
- `npte netns run` is parsed with `vflag.DisablePermute`, so any
  flag-like token in `input.Argv` (which already sits *after* the
  netns positional) lands as an inner-command positional. This is
  defense-in-depth on top of `--`; `--` alone is sufficient because
  it terminates flag parsing for everything that follows.

All three are load-bearing. Removing the injection in `netns.go`
widens the agent's reach to "anything `$SUDO_USER` could do inside
the namespace." Removing the `--` reopens the bypass via
`input.Netns`. Allowing flag permutation upstream re-introduces a
worse version of the original bug (every positional becomes a
candidate flag again). Each change is a security-relevant edit;
treat it as such and audit accordingly.

### 3. Argv composition is structured, not free-form

The MCP layer accepts structured tool inputs (typed JSON schemas in
`netns.go`, `netem.go`, `lab.go`) and assembles argv from them. Two
rules apply:

- **No string concatenation of user-supplied bytes into a single argv
  element.** Each value lands as its own argv token (e.g., `--rate`,
  `10mbit` are two tokens). The `netem.go` `for kv := range ...` loop
  is the template. Do not introduce `fmt.Sprintf("--rate=%s", v)` or
  similar — it makes argv-level boundaries fuzzier and breaks the
  one-token-one-value contract that the npte-side validators
  assume.

- **Defer validation to the npte side where possible.** The MCP
  layer is a thin schema-to-argv adapter; the authoritative
  validators live in `internal/validate` and run inside the forked
  `npte` process. Tool input schemas should describe the shape
  (string, list, map) and document the grammar in the `jsonschema`
  tag, not re-implement validation. This keeps the validation rules
  in one place and ensures the agent gets a consistent error
  message via `stderr.txt` regardless of whether it bypassed the
  MCP and shelled to `npte` directly.

- **Every agent-supplied positional sits after a GNU `--`
  separator.** vflag (like every getopt-style parser) treats `--`
  as "end of flags": tokens after it are positionals regardless of
  leading dashes. Each tool handler that takes a positional
  (`start_netns_run`, `netns_show`, `shape_download`, `shape_upload`,
  `shape_clear`) inserts
  `--` before the first user-supplied positional in the argv
  composition. This is the structural guarantee that no field of
  an input schema can flip a flag the MCP just set — see invariant
  #2 for the concrete case (sandbox bypass) and the historical bug
  that produced this rule. `DisablePermute` on the npte side is
  defense-in-depth, not the primary boundary.

The exception is the `start_netns_run` `Env` field: the MCP layer
sorts entries before serializing to `-e KEY=VALUE` tokens so that the
argv is deterministic (and the `argv.json` session file is
reproducible).

### 4. Server `instructions` is authoritative; per-tool descriptions
are a fallback

The handshake-time `instructions` block in `serve.go` ships the
trust-bridge framing, session layout, sandbox-escape rule, and
process-lifecycle contract once per session. Per-tool descriptions
carry only what is specific to that tool. When editing either:

- Cross-cutting framing (trust model, sandbox-escape rule, lifecycle
  contract) belongs in `instructions`. Do not duplicate it into
  per-tool descriptions — duplication invites drift.
- Per-tool descriptions MAY repeat the short preamble ("See
  server instructions for trust model and session layout.") as a hint
  to clients that do not surface `instructions` to the LLM. They MUST
  NOT contradict `instructions`. Single-step synchronous tools use
  `runPreamble`; multi-step synchronous tools (shape_*) use
  `shapePreamble`; `start_netns_run` uses `startPreamble`.
- The resolved `absSessionDir` is substituted into `instructions` at
  startup so the agent learns the session path declaratively rather
  than from string surgery. Keep it there; do not add a
  `get_session_dir` tool.

### 5. The MCP server itself drops nothing

`npte mcp serve` runs as `$USER`, not root. It cannot directly touch
kernel state — every privileged operation goes through `sudo -n npte`.
Do not add code paths in this package that exec, mount, or otherwise
require elevated capabilities directly. If a feature seems to need
those, the work belongs in a sudoers-allowlisted verb, not here.

### 6. Shape tools hardcode the canonical shaping targets

`shape_download`, `shape_upload`, and `shape_clear` operate on the
canned lab's canonical shaping targets: `router:if-client` (download
path, router→client egress) and `router:if-server` (upload path,
router→server egress). The target namespace and interface are
hardcoded in the handler implementations — the agent never supplies
them, so there is no validation step and no opportunity for the agent
to shape at a wrong location.

`shape_download` and `shape_upload` each clear the target interface
before applying, so they are always safe to call regardless of prior
state. `shape_clear` clears both interfaces.

Out of scope on purpose:

- `start_netns_run` and `netns_show` accept `client`,
  `router`, and `server` freely — iperf3 and friends must run
  inside the leaves. The constraint is only on shape-side
  handlers, because that is the only place where a
  topologically-wrong choice produces silently-wrong results.
- A human driving `npte netem ...` from a shell can shape
  anywhere they like. The MCP layer is opinionated; the
  underlying primitive is not.

## When editing this package

1. **Adding a new tool.** Confirm the target verb is in
   the sudoers allowlist (`internal/cli/sudoers/sudoers.go`).
   Confirm the target package holds its own audit invariants
   (`internal/cli/<name>/CLAUDE.md`). Add a typed input schema with
   `jsonschema` tags describing the grammar; do not re-validate on
   the MCP side. **If the tool takes any user-supplied positional
   argument, insert a GNU `--` separator immediately before the
   first such argument in the argv composition** — without it, that
   positional can still be parsed as a flag (see invariants #2 and
   #3).

2. **Editing `start_netns_run`.** Touch invariant #2. Read it twice.
   Verify that `--sandbox` injection, the `--` separator before
   `input.Netns`, and `vflag.DisablePermute` upstream all remain in
   place. Add a regression test if the change is non-trivial.

3. **Editing `serve.go`'s `instructions` or tool descriptions.**
   Re-read invariant #4. Keep cross-cutting framing in
   `instructions`; keep per-tool descriptions specific.

4. **Editing `shape_download`, `shape_upload`, or `shape_clear`.**
   Touch invariant #6. The target namespace and interface are
   hardcoded in `shapeApply` and `ShapeClear` — verify they still
   match the canned lab's naming (`router`, `if-client`,
   `if-server`). If `lab create`'s naming ever changes in
   `internal/cli/lab/`, update the hardcoded strings in `netem.go`
   AND the tool-semantics paragraph in `serve.go`'s instructions
   block in the same change.

5. **Editing `startProc`.** It is the choke point for every
   privileged invocation. Changes that affect argv composition (the
   `[]string{"/usr/bin/sudo", "-n", sm.exePath}` prefix, sudo
   non-interactivity, the session layout) are security-relevant. The
   `-n` flag is what turns "verb is outside the allowlist" into a
   clean exit-non-zero failure rather than a hung password prompt.

## References

- `internal/cli/sudoers/sudoers.go` — emits the NOPASSWD snippet
  that defines which verbs the MCP can fork. If you change which
  subcommand classes are allowlisted, also revisit this package's
  exported tool surface.
- `internal/cli/sudoers/CLAUDE.md` — the audit invariant for the
  NOPASSWD side of the boundary.
- `internal/cli/netns/CLAUDE.md`,
  `internal/cli/netem/CLAUDE.md`,
  `internal/cli/lab/CLAUDE.md` — registry and validator contracts
  the MCP relies on by inheritance.
- `internal/cli/tutorial/chapters/110-mcp.md` — user-facing
  description of the trust bridge, the `.mcp.json` wiring, the
  toolbox, and the session layout.
- `internal/cli/tutorial/chapters/100-sandbox.md` — the bubblewrap
  policy that invariant #2 propagates to the agent.
