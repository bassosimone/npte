// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serveMain is the main of the `mcp serve` subcommand.
func serveMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte mcp serve", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Speak the Model Context Protocol over stdio, exposing npte's "+
			"privileged primitives as MCP tools. Most tools run "+
			"synchronously; `start_netns_run` starts a background "+
			"process paired with `wait` and `kill`. Intended to be "+
			"wired as a stdio MCP server in an agent's `.mcp.json`. "+
			"Experimental.",
		"Trust model: the MCP server runs outside the agent's sandbox "+
			"and is the agent's only authorized channel for invoking "+
			"npte; the privileged side is kept safe by npte itself "+
			"(privilege drop, per-command sandboxing). Requires "+
			"passwordless sudo for /usr/bin/sudo; see `npte sudoers`.",
		"Per-invocation artifacts (argv.json, stdout.txt, stderr.txt, "+
			"exitcode.txt) are written under "+
			"./.npte/sessions/<sessionId>/<procId>/ in the current "+
			"working directory; <sessionId> is a UUIDv7 minted at "+
			"`mcp serve` startup, so successive invocations get fresh, "+
			"chronologically sortable directories. The MCP server only "+
			"writes these files; it never reads them back, so the agent "+
			"is free to inspect or modify them.",
	)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	runtimex.PanicOnError0(fset.Parse(args))

	exe, err := env.Executable()
	env.LogFatalOnError0(err)
	sessionID := runtimex.PanicOnError1(uuid.NewV7()).String()
	absSessionDir, err := filepath.Abs(filepath.Join(".npte", "sessions", sessionID))
	env.LogFatalOnError0(err)
	err = env.MkdirAll(absSessionDir, 0700)
	env.LogFatalOnError0(err)
	mgr := newSessionManager(env, exe, absSessionDir)

	// Server-level instructions shipped to the client at handshake. Per
	// the MCP spec, clients MAY surface these as a hint to the LLM
	// (typically appended to the system prompt). Putting the shared
	// trust-bridge framing, sandbox-escape rule, session layout, and
	// process-lifecycle contract here means we pay for them once at
	// session start instead of in every per-tool description.
	instructions := "npte exposes privileged network-namespace primitives " +
		"to agents that cannot shell to sudo directly. This server is " +
		"a trust bridge, not a sandbox: it runs outside the agent's " +
		"sandbox and relies on npte's own privilege drop and per-command " +
		"sandboxing.\n\n" +
		"Session root for this invocation: " + absSessionDir + "\n" +
		"Every tool call writes argv.json, stdout.txt, stderr.txt, " +
		"and exitcode.txt to <session_root>/<procId>/. Read these via " +
		"Bash; the MCP never reads them back. procIds are UUIDv7, so " +
		"`ls <session_root>/` is chronological. To enumerate every " +
		"command run this session with its exit code:\n\n" +
		"    for d in <session_root>/*/; do\n" +
		"      pid=$(basename \"$d\")\n" +
		"      ec=$(cat \"$d/exitcode.txt\" 2>/dev/null || echo running)\n" +
		"      printf '%s  %s  %s\\n' \"$pid\" \"$ec\" \"$(cat \"$d/argv.json\")\"\n" +
		"    done\n\n" +
		"Filter on $ec for failures only. Prefer this one-liner over " +
		"asking the user to recall what ran. The parent directory " +
		"(./.npte/sessions/) holds one subdirectory per `mcp serve` " +
		"invocation, also UUIDv7-named; prior sessions are not " +
		"reachable from this server but their on-disk contents persist " +
		"for archival.\n\n" +
		"Sandbox-escape rule: use an MCP tool ONLY when your sandbox " +
		"prevents you from invoking npte via Bash. For non-privileged " +
		"operations (e.g., `npte --help`, `npte doctor`, `npte tutorial`, " +
		"or reading files under <session_root>) you MUST prefer Bash.\n\n" +
		"Lab shaping convention. After `lab_create`, the topology is " +
		"three namespaces: `client`, `router`, `server`. Veths are " +
		"`client.if-router <-> router.if-client` and " +
		"`server.if-router <-> router.if-server`. For all path-level " +
		"shaping (delay, rate, loss, AQM experiments), netem MUST be " +
		"installed at the router: `router if-client` for router->client " +
		"egress and `router if-server` for router->server egress. Both " +
		"directions live at the hub, which is the canonical bottleneck. " +
		"Do NOT apply netem to a leaf-side interface " +
		"(`client if-router`, `server if-router`); the path's " +
		"bottleneck does not live there and the resulting throughput " +
		"numbers will not correspond to any reproducible scenario.\n\n" +
		"Tool semantics. Most tools (netns_list, netns_show, " +
		"netem_apply, netem_clear, lab_create, lab_destroy) run " +
		"synchronously: the call blocks until the underlying npte " +
		"command finishes (≤30 s timeout) and the exit code is " +
		"returned inline. No wait or kill needed.\n\n" +
		"start_netns_run is the exception: it starts a long-lived " +
		"background process and returns immediately with a procId. " +
		"Pair every successful start_netns_run with `wait` on the " +
		"returned procId. Use `kill` to send SIGINT. A `wait` " +
		"returning terminated=true reaps the proc (one-shot); subsequent " +
		"wait/kill on the same procId fail with \"no such process\". The " +
		"procDir survives the reap."

	// TODO(bassosimone): need to figure out a way to share the version across
	// multiple components, probably ./internal/version/version.go.
	server := mcp.NewServer(
		&mcp.Implementation{Name: "npte", Version: "v0.5.0"},
		&mcp.ServerOptions{Instructions: instructions},
	)

	// Preamble for synchronous tools that run a privileged npte
	// invocation to completion and return the result inline.
	const runPreamble = "Run a privileged npte invocation " +
		"synchronously. Returns the exit code and session directory " +
		"directly; no wait or kill needed. See server instructions " +
		"for trust model and session layout. "

	// Preamble for start_netns_run, which is the only tool that
	// starts a long-lived background process requiring wait/kill.
	const startPreamble = "Start a privileged npte invocation as a " +
		"background process. See server instructions for trust model, " +
		"session layout, and process lifecycle. "

	mcp.AddTool(server, &mcp.Tool{
		Name: "netns_list",
		Description: runPreamble +
			"This tool runs `npte netns list`, which writes one " +
			"npte-managed network namespace name per line to stdout " +
			"(stdout is empty if there are none).",
	}, mgr.NetnsList)
	mcp.AddTool(server, &mcp.Tool{
		Name: "netns_show",
		Description: runPreamble +
			"This tool runs `npte netns show`, which dumps a fixed " +
			"set of diagnostic sections for a single npte-managed " +
			"namespace (link, addr, route, route6, qdisc, neigh, " +
			"sockets, pids) to stdout, each preceded by a `=== " +
			"<section> ===` header in canonical order. Use the " +
			"`sections` field to restrict the dump; if omitted, all " +
			"sections are emitted.",
	}, mgr.NetnsShow)
	mcp.AddTool(server, &mcp.Tool{
		Name: "start_netns_run",
		Description: startPreamble +
			"This tool runs a command inside an npte-managed network " +
			"namespace. Privilege is dropped back to the invoking " +
			"user (via runuser), and the command is ALWAYS wrapped " +
			"in npte's bubblewrap sandbox: the host filesystem is " +
			"read-only at /, the MCP server's working directory is " +
			"rebound read-write, /tmp is a fresh tmpfs, /proc and " +
			"/dev are freshly mounted, and PID/IPC/UTS namespaces " +
			"are unshared. The MCP enforces --sandbox " +
			"unconditionally; there is no way for an agent to " +
			"disable it. The `argv` field is the command to invoke " +
			"inside the namespace (first element is the program); " +
			"`env` sets environment variables.",
	}, mgr.NetnsRun)
	mcp.AddTool(server, &mcp.Tool{
		Name: "netem_apply",
		Description: runPreamble +
			"This tool runs `npte netem apply`, which installs " +
			"`root handle 1: netem` on the given interface inside " +
			"the given namespace with the requested shaping knobs. " +
			"At least one of delay/loss/limit/rate/slot/child must " +
			"be set. Flag values are passed verbatim to tc; see `man " +
			"tc-netem` for the value grammar. NOT idempotent: " +
			"re-applying on an already-shaped interface fails. Clear " +
			"first with `netem_clear`.",
	}, mgr.NetemApply)
	mcp.AddTool(server, &mcp.Tool{
		Name: "netem_clear",
		Description: runPreamble +
			"This tool runs `npte netem clear`, which removes the " +
			"root qdisc (and any child attached at parent 1:) from " +
			"the given interface inside the given namespace. " +
			"Idempotent: tolerated if no qdisc is present.",
	}, mgr.NetemClear)
	mcp.AddTool(server, &mcp.Tool{
		Name: "lab_create",
		Description: runPreamble +
			"This tool runs `npte lab create`, which creates npte's " +
			"canned three-node lab topology: leaf namespaces `client` " +
			"and `server`, each sharing a veth pair with hub " +
			"namespace `router`; leaf default routes via the router. " +
			"Addresses are fixed in 172.16.0.0/16 " +
			"(client↔router=172.16.3.0/24, " +
			"server↔router=172.16.2.0/24). Off-link traffic is NOT " +
			"wired (no host uplink, no NAT).",
	}, mgr.LabCreate)
	mcp.AddTool(server, &mcp.Tool{
		Name: "lab_destroy",
		Description: runPreamble +
			"This tool runs `npte lab destroy`, which tears down the " +
			"canned three-node lab (destroys the `client`, `server`, " +
			"and `router` namespaces). Does NOT touch host-side " +
			"gateway state.",
	}, mgr.LabDestroy)

	mcp.AddTool(server, &mcp.Tool{
		Name: "wait",
		Description: "Wait for a process started by start_netns_run " +
			"to terminate, up to the specified timeout. Returns " +
			"`terminated=true` and the exit code if the process " +
			"finished within the timeout; returns `terminated=false` " +
			"(and a meaningless `exitCode`) if the timeout elapsed " +
			"first — in that case call `wait` again. Reap is one-shot: " +
			"once `wait` returns `terminated=true`, the process is " +
			"removed from the MCP's in-memory table and subsequent " +
			"`wait`/`kill` calls for the same procId will fail with " +
			"\"no such process\". The procDir survives the reap; read " +
			"`exitcode.txt`, `stdout.txt`, and `stderr.txt` from " +
			"procDir via Bash if you need the data later.",
	}, mgr.Wait)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kill",
		Description: "Send a signal to a process started by " +
			"start_netns_run. Currently only SIGINT (\"INT\" or " +
			"\"SIGINT\") is supported; SIGKILL cannot be relayed " +
			"through sudo and is intentionally not exposed. Returns " +
			"an error if the process has already terminated or has " +
			"been reaped by a successful `wait`. After `kill` you " +
			"still need to call `wait` to observe termination and " +
			"reap the process.",
	}, mgr.Kill)

	// Run the server until there's an error
	err1 := server.Run(ctx, &mcp.StdioTransport{})

	// Make sure we kill process that are still running
	err2 := serveCleanup(mgr)

	// Exit on failure to run the server or to cleanup
	env.LogFatalOnError0(errors.Join(err1, err2))
	return nil
}

func serveCleanup(mgr *sessionManager) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return mgr.Cleanup(ctx)
}
