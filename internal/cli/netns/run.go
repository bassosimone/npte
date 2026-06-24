// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"math"
	"strings"
	"sync"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/registry"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// runMain is the main of the `netns run` subcommand.
//
// Execution strategy: `ip netns exec <ns> runuser -u <user> -- env K=V... <cmd> [args...]`.
//
// Why this pipeline rather than `systemd-run --property=NetworkNamespacePath=...`:
//
//  1. The `/etc/netns/<ns>/*` → `/etc/*` overlay (which gives the namespace
//     its own resolv.conf, installed by `netns create`) is not a property
//     of the network namespace. It is implemented in userspace by
//     `ip netns exec` itself: setns, unshare(CLONE_NEWNS), rslave /, then
//     bind-mount each file under `/etc/netns/<ns>/` over the corresponding
//     `/etc/<file>`. Any tool that enters the netns by a different path
//     (systemd-run via NetworkNamespacePath, nsenter, direct setns) gets
//     none of this and sees the host's `/etc/resolv.conf`. Going through
//     `ip netns exec` is the only way to inherit the overlay for free.
//
//  2. `ip netns exec` spawns the target as an honest child of the invoking
//     shell: it keeps the controlling TTY, job control works (^C, ^Z,
//     jobs/fg/bg), `ps` under the caller's tty shows it, SIGHUP flows from
//     the terminal, and `wait(2)` returns the real exit status.
//     `systemd-run` routes through PID 1 via D-Bus; the command then runs
//     as a transient unit under init, with stdio proxied and signals
//     relayed. That is the wrong shape for an interactive "step into the
//     namespace and poke at it" command, which is the primary use case.
//
// `runuser` (util-linux) drops privilege from root to <user> without PAM
// prompts or sudoers config; `env` (coreutils) injects the requested
// KEY=VALUE pairs after the privilege drop. Both are universally available
// on target platforms.
//
// An alternative worth naming: `systemd-run --pipe --quiet --collect
// --property=NetworkNamespacePath=/run/netns/<ns>
// --property=BindPaths=/etc/netns/<ns>/resolv.conf:/etc/resolv.conf
// --uid=<user> --setenv=K=V -- <cmd>`. This form is preferable when the
// workload is long-running and benefits from a transient cgroup-scoped
// unit (per-run CPU/memory/IO accounting, `systemctl status`,
// `journalctl -u`, detachment from the invoking shell, `MemoryMax=` /
// `CPUQuota=` per invocation, atomic kill of the whole unit). It is a
// poor fit for interactive work because it gives up the shell-child
// semantics above. If that workload ever needs a first-class home, it
// belongs in a separate subcommand that advertises the detachment and
// the unit name rather than hiding a D-Bus call inside `run`.
func runMain(ctx context.Context, args []string) error {
	env := testable.Env

	var (
		envFlags []string
		dryRun   bool
		sandbox  bool
	)

	fset := vflag.NewFlagSet("npte netns run", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Runs a command inside a network namespace. The command is an honest "+
			"child of the invoking shell: it keeps the controlling TTY and job "+
			"control works as expected.",
		"Entry is via `ip netns exec`, which also bind-mounts every file under "+
			"/etc/netns/<ns>/ over the corresponding /etc/ path, so the namespace-"+
			"scoped resolv.conf installed by `npte netns create` is in effect.",
		"The command runs as $SUDO_USER (the user who invoked sudo), which "+
			"must be set in the environment — running via sudo does this "+
			"automatically. This scopes any damage the inner command can "+
			"do to the invoking user's account. Use -e/--env to inject "+
			"KEY=VALUE pairs; the inner process otherwise starts with "+
			"runuser(1)'s default environment for the target user.",
		"With --sandbox, the inner command is additionally wrapped in "+
			"bubblewrap: the host filesystem is mounted read-only at /, "+
			"the current working directory is rebound read-write, /tmp is "+
			"a fresh tmpfs, and /proc and /dev are freshly mounted. The "+
			"PID, IPC, and UTS namespaces are unshared from the host; the "+
			"network namespace is explicitly shared so the command sees "+
			"the netns entered by `ip netns exec`. Caveats: --sandbox is "+
			"an integrity boundary, not a confidentiality one — anything "+
			"$SUDO_USER could read on the host stays readable inside the "+
			"sandbox; setuid elevation is disabled (PR_SET_NO_NEW_PRIVS); "+
			"and writes outside the current directory — including caches "+
			"and configs under $HOME — will see EROFS.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run. The script sets no shell "+
			"options of its own; wrap it (e.g. with `set -euxo pipefail`) "+
			"if you want fail-fast semantics.",
	)
	usage.PositionalArgumentsUsage = "<ns> <command> [args...]"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.StringSliceVar(&envFlags, 'e', "env", "Set environment variable `KEY=VALUE` (repeatable).")
	fset.BoolVar(&sandbox, 0, "sandbox", "Wrap the command in a bubblewrap sandbox with the host filesystem read-only except the current working directory.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args)) // cannot fail: using vflag.ExitOnError

	// NOPASSWD audit invariant: this command is part of the set that
	// `npte sudoers` allowlists for sudo execution without a password
	// (see CLAUDE.md in this package). Every flag value, positional, or
	// environment value forwarded to a subprocess must be validated
	// here — fail loud, prefer hardcoded literals, never trust the
	// caller's bytes. A missing check is a passwordless privesc hole.

	ns := fset.Args()[0]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte netns run: %s", err)
		env.Exit(2)
		return nil
	}

	// $SUDO_USER identifies who to drop privileges back to. Sudo derives
	// it from its own getuid()+getpwuid() and writes it unconditionally
	// (sudo-managed, not env-passthrough), so we trust the value; we
	// only check that it is present and well-formed so that a missing
	// or malformed value fails loud here rather than as a confusing
	// downstream error from runuser(1). The privilege boundary is
	// enforced by sudo (who is allowed to run npte as root) and by the
	// kernel (who has the capabilities `ip netns exec` requires); we do
	// not re-litigate it.
	userFlag := env.Getenv("SUDO_USER")
	if userFlag == "" {
		logx.Error("npte netns run: $SUDO_USER is not set; " +
			"this command must be invoked via sudo so that it can drop " +
			"privileges back to the invoking user")
		env.Exit(2)
		return nil
	}
	if err := validate.Username(userFlag); err != nil {
		logx.Error("npte netns run: $SUDO_USER: %s", err)
		env.Exit(2)
		return nil
	}
	for _, ev := range envFlags {
		key, _, ok := strings.Cut(ev, "=")
		if !ok {
			logx.Error("npte netns run: -e value must be KEY=VALUE, got %q", ev)
			env.Exit(2)
			return nil
		}
		if err := validate.EnvVarName(key); err != nil {
			logx.Error("npte netns run: -e: %s", err)
			env.Exit(2)
			return nil
		}
	}

	// Unlike every other verb in this package, `run` performs no
	// marker operation — it just enters the namespace and execs the
	// user's command, which may be a long-running server. Holding
	// the global registry lock across that exec would block every
	// other npte invocation, most importantly a sibling `npte netns
	// run` for the client side of the same experiment.
	//
	// We therefore take the lock only across the RequireManaged
	// check (which is what the registry invariant actually requires:
	// the ownership predicate must be observed atomically with
	// respect to concurrent create/destroy) and release it before
	// exec. The privesc bound still holds: we confirmed under the
	// lock that the namespace is npte-managed, and a concurrent
	// destroy can at worst make `ip netns exec` fail with ENOENT —
	// it cannot redirect us into a namespace npte does not own,
	// since recreating a managed name requires `netns create`,
	// which itself takes the lock.
	//
	// The unlock function returned by registry.MustLock is not
	// idempotent (the underlying lockedfile.Mutex panics on a
	// double Unlock), so we wrap it in sync.OnceFunc to make the
	// "release early, also release on any error path" pattern
	// safe: the explicit unlock before MustRun is the common case,
	// and the deferred unlock is the safety net for the early-exit
	// branches above (including future ones).
	unlock := sync.OnceFunc(registry.MustLock(ctx, env, dryRun))
	defer unlock()

	if err := registry.RequireManaged(env, dryRun, ns); err != nil {
		logx.Error("npte netns run: %s", err)
		env.Exit(2)
		return nil
	}

	// Start assembling: ip netns exec <ns> runuser -u <user> --
	argv := []string{"netns", "exec", ns, "runuser", "-u", userFlag, "--"}

	// Optionally chain with bwrap to sandbox the command
	if sandbox {
		// Load the working directory. No validator needed since the
		// working directory is obtained via a syscall.
		workDir, err := env.Getwd()
		env.LogFatalOnError0(err)

		// Assemble bwrap command to create the actual sandbox
		argv = append(argv, "bwrap")
		argv = append(argv, "--ro-bind", "/", "/")
		argv = append(argv, "--tmpfs", "/tmp")
		argv = append(argv, "--proc", "/proc")
		argv = append(argv, "--dev", "/dev")
		argv = append(argv, "--bind", workDir, workDir)
		argv = append(argv, "--chdir", workDir)
		argv = append(argv, "--share-net")
		argv = append(argv, "--unshare-pid", "--unshare-ipc", "--unshare-uts")
		argv = append(argv, "--die-with-parent")
		argv = append(argv, "--")
	}

	// Add zero or more direct env assignments
	argv = append(argv, "env")
	argv = append(argv, envFlags...)

	// Add the command to execute
	argv = append(argv, fset.Args()[1:]...)

	// Release the registry lock before exec; see the comment above.
	unlock()
	logx.Details("npte: enter namespace %q as user %q", ns, userFlag)
	subprocess.MustRun(ctx, dryRun, "ip", argv...)
	return nil
}
