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
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serveMain is the main of the `mcp serve` subcommand.
func serveMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte mcp serve", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Speak the Model Context Protocol over stdio, exposing npte's "+
			"privileged primitives as the `start`, `wait`, and `kill` "+
			"tools. Intended to be wired as a stdio MCP server in an "+
			"agent's `.mcp.json`. Experimental.",
		"Trust model: the MCP server runs outside the agent's sandbox "+
			"and is the agent's only authorized channel for invoking "+
			"npte; the privileged side is kept safe by npte itself "+
			"(privilege drop, per-command sandboxing). Requires "+
			"passwordless sudo for /usr/bin/sudo; see `npte sudoers`.",
		"Per-invocation artifacts (argv.json, stdout.txt, stderr.txt, "+
			"exitcode.txt) are written under ./.npte/spool/<uuid>/ in "+
			"the current working directory. The MCP server only writes "+
			"these files; it never reads them back, so the agent is "+
			"free to inspect or modify them.",
	)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	runtimex.PanicOnError0(fset.Parse(args))

	exe, err := env.Executable()
	env.LogFatalOnError0(err)
	spoolDir := filepath.Join(".npte", "spool")
	absSpoolDir, err := filepath.Abs(spoolDir)
	env.LogFatalOnError0(err)
	err = env.MkdirAll(absSpoolDir, 0700)
	env.LogFatalOnError0(err)
	mgr := newSessionManager(env, exe, absSpoolDir)

	// Server-level instructions shipped to the client at handshake. Per
	// the MCP spec, clients MAY surface these as a hint to the LLM
	// (typically appended to the system prompt). Putting the shared
	// trust-bridge framing, sandbox-escape rule, spool layout, and
	// process-lifecycle contract here means we pay for them once at
	// session start instead of in every per-tool description.
	instructions := "npte exposes privileged network-namespace primitives " +
		"to agents that cannot shell to sudo directly. This server is " +
		"a trust bridge, not a sandbox: it runs outside the agent's " +
		"sandbox and relies on npte's own privilege drop and per-command " +
		"sandboxing.\n\n" +
		"Spool root for this session: " + absSpoolDir + "\n" +
		"Every start_* call writes argv.json, stdout.txt, stderr.txt, " +
		"and (after termination) exitcode.txt to <spool_root>/<procId>/. " +
		"Read these via Bash; the MCP never reads them back.\n\n" +
		"Sandbox-escape rule: use a start_* tool ONLY when your sandbox " +
		"prevents you from invoking npte via Bash. For non-privileged " +
		"operations (e.g., `npte --help`, `npte doctor`, `npte tutorial`, " +
		"or reading files under <spool_root>) you MUST prefer Bash.\n\n" +
		"Process lifecycle: pair every successful start_* with `wait` " +
		"on the returned procId. Use `kill` to send SIGINT. A `wait` " +
		"returning terminated=true reaps the proc (one-shot); subsequent " +
		"wait/kill on the same procId fail with \"no such process\". The " +
		"on-disk spool directory survives the reap."

	// TODO(bassosimone): need to figure out a way to share the version across
	// multiple components, probably ./internal/version/version.go.
	server := mcp.NewServer(
		&mcp.Implementation{Name: "npte", Version: "v0.5.0"},
		&mcp.ServerOptions{Instructions: instructions},
	)

	// Common preamble for every start_* tool description. Kept short
	// because the trust-bridge framing and lifecycle contract live in
	// server-level instructions (above); this is the fallback for
	// clients that don't surface instructions to the LLM.
	const startPreamble = "Start a privileged npte invocation as a " +
		"background process. See server instructions for trust model, " +
		"spool layout, and process lifecycle. "

	mcp.AddTool(server, &mcp.Tool{
		Name: "start_netns_list",
		Description: startPreamble +
			"This tool runs `npte netns list`, which writes one " +
			"npte-managed network namespace name per line to stdout " +
			"(stdout is empty if there are none).",
	}, mgr.NetnsList)
	mcp.AddTool(server, &mcp.Tool{
		Name: "start_netns_show",
		Description: startPreamble +
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
		Name: "start_netem_apply",
		Description: startPreamble +
			"This tool runs `npte netem apply`, which installs " +
			"`root handle 1: netem` on the given interface inside " +
			"the given namespace with the requested shaping knobs. " +
			"At least one of delay/loss/limit/rate/slot/child must " +
			"be set. Flag values are passed verbatim to tc; see `man " +
			"tc-netem` for the value grammar. NOT idempotent: " +
			"re-applying on an already-shaped interface fails. Clear " +
			"first with `start_netem_clear`.",
	}, mgr.NetemApply)
	mcp.AddTool(server, &mcp.Tool{
		Name: "start_netem_clear",
		Description: startPreamble +
			"This tool runs `npte netem clear`, which removes the " +
			"root qdisc (and any child attached at parent 1:) from " +
			"the given interface inside the given namespace. " +
			"Idempotent: tolerated if no qdisc is present.",
	}, mgr.NetemClear)
	mcp.AddTool(server, &mcp.Tool{
		Name: "start_lab_create",
		Description: startPreamble +
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
		Name: "start_lab_destroy",
		Description: startPreamble +
			"This tool runs `npte lab destroy`, which tears down the " +
			"canned three-node lab (destroys the `client`, `server`, " +
			"and `router` namespaces). Does NOT touch host-side " +
			"gateway state.",
	}, mgr.LabDestroy)

	mcp.AddTool(server, &mcp.Tool{
		Name: "wait",
		Description: "Wait for a process started by any start_* tool " +
			"to terminate, up to the specified timeout. Returns " +
			"`terminated=true` and the exit code if the process " +
			"finished within the timeout; returns `terminated=false` " +
			"(and a meaningless `exitCode`) if the timeout elapsed " +
			"first — in that case call `wait` again. Reap is one-shot: " +
			"once `wait` returns `terminated=true`, the process is " +
			"removed from the MCP's in-memory table and subsequent " +
			"`wait`/`kill` calls for the same procId will fail with " +
			"\"no such process\". The on-disk spool directory survives " +
			"the reap; read `exitcode.txt`, `stdout.txt`, and " +
			"`stderr.txt` from procDir via Bash if you need the data " +
			"later.",
	}, mgr.Wait)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kill",
		Description: "Send a signal to a process started by any " +
			"start_* tool. Currently only SIGINT (\"INT\" or " +
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
