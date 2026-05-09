// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sandboxBwrapPolicy is the literal bwrap argv segment emitted when --sandbox
// is enabled. Pinning it here lets the table-driven tests assert call-shape
// without re-deriving the policy in every case. The workDir in --bind/--chdir
// matches the testenv.Setup stub for Getwd.
const sandboxBwrapPolicy = "bwrap " +
	"--ro-bind / / " +
	"--tmpfs /tmp " +
	"--proc /proc " +
	"--dev /dev " +
	"--bind /home/sbs/src/github.com/bassosimone/npte /home/sbs/src/github.com/bassosimone/npte " +
	"--chdir /home/sbs/src/github.com/bassosimone/npte " +
	"--share-net " +
	"--unshare-pid --unshare-ipc --unshare-uts " +
	"--die-with-parent " +
	"--"

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		sudoUser string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run happy path",
		sudoUser: "alice",
		args:     []string{"--dry-run", "client", "ip", "addr"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client runuser -u alice -- env ip addr",
		},
	}, {
		name:     "dry-run with env vars",
		sudoUser: "alice",
		args:     []string{"--dry-run", "-e", "FOO=bar", "client", "ip", "addr"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client runuser -u alice -- env FOO=bar ip addr",
		},
	}, {
		name:     "rejects bad ns",
		sudoUser: "alice",
		args:     []string{"--dry-run", "1bad", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "rejects missing SUDO_USER",
		sudoUser: "",
		args:     []string{"--dry-run", "client", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "rejects malformed SUDO_USER",
		sudoUser: "1bad",
		args:     []string{"--dry-run", "client", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "rejects bad env var",
		sudoUser: "alice",
		args:     []string{"--dry-run", "-e", "1BAD=x", "client", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "rejects env without =",
		sudoUser: "alice",
		args:     []string{"--dry-run", "-e", "noequals", "client", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "dry-run with sandbox",
		sudoUser: "alice",
		args:     []string{"--dry-run", "--sandbox", "client", "ip", "addr"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client runuser -u alice -- " + sandboxBwrapPolicy + " env ip addr",
		},
	}, {
		// Pins that -e KEY=VALUE pairs land inside bwrap's inner command
		// (after bwrap's --), not before bwrap. Placement matters: env vars
		// before bwrap would set them in bwrap's own environment rather
		// than in the sandboxed inner process.
		name:     "dry-run with sandbox and env",
		sudoUser: "alice",
		args:     []string{"--dry-run", "--sandbox", "-e", "FOO=bar", "client", "ip", "addr"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client runuser -u alice -- " + sandboxBwrapPolicy + " env FOO=bar ip addr",
		},
	}, {
		// vflag invariant 1: `--` terminates flag parsing.
		// `--sandbox` after `--` must be a positional, not a flag — so the
		// inner command literally contains `--sandbox` and bwrap is absent.
		// If `--` were ignored, the output would include the bwrap policy.
		name:     "vflag invariant: -- terminates flag parsing",
		sudoUser: "alice",
		args:     []string{"--dry-run", "--", "client", "echo", "--sandbox"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client runuser -u alice -- env echo --sandbox",
		},
	}, {
		// vflag invariant 2: DisablePermute=true causes the
		// first non-flag positional to end flag parsing. `--sandbox` after
		// `client` must be a positional, even without `--`. Same observable
		// signal as invariant 1: bwrap absent, `--sandbox` literal in inner.
		name:     "vflag invariant: DisablePermute stops at first positional",
		sudoUser: "alice",
		args:     []string{"--dry-run", "client", "--sandbox", "echo", "hello"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client runuser -u alice -- env --sandbox echo hello",
		},
	}, {
		// vflag invariant 3: bool long flags do NOT consume
		// the next argv element as their value. `--sandbox true` must parse
		// as `--sandbox` (=true via DefaultValue) followed by positional
		// `true`. Distinguishing signal: ns name is `true`, not `client`.
		// If --sandbox had consumed "true", positionals would be [client,
		// echo] and ns=client; this would mask the injection vector the
		// MCP threat model worries about.
		name:     "vflag invariant: bool long flag rejects separate-arg",
		sudoUser: "alice",
		args:     []string{"--dry-run", "--sandbox", "true", "echo", "hello"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/true || { echo 'npte: true: not managed by npte' >&2; exit 2; }`,
			"ip netns exec true runuser -u alice -- " + sandboxBwrapPolicy + " env echo hello",
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			s.SudoUser = tc.sudoUser
			require.NoError(t, runMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}

// TestRun_GetwdGatedBySandbox pins that Getwd is called only when --sandbox
// is set. The non-sandbox path inherits CWD transparently through `ip netns
// exec → runuser`, so calling Getwd would be a needless new failure mode (a
// CWD that has been deleted under the user — `rm -rf $(pwd)` — would crash
// runs that previously worked). Regression guard for the conditional.
func TestRun_GetwdGatedBySandbox(t *testing.T) {
	t.Run("not called without --sandbox", func(t *testing.T) {
		s := testenv.Setup(t)
		s.SudoUser = "alice"
		getwdCalls := 0
		testable.Env.Getwd = func() (string, error) {
			getwdCalls++
			return "/home/sbs/src/github.com/bassosimone/npte", nil
		}

		require.NoError(t, runMain(context.Background(),
			[]string{"--dry-run", "client", "ip", "addr"}))

		assert.Equal(t, -1, s.ExitCode)
		assert.Equal(t, 0, getwdCalls, "Getwd must not be called without --sandbox")
	})

	t.Run("called with --sandbox", func(t *testing.T) {
		s := testenv.Setup(t)
		s.SudoUser = "alice"
		getwdCalls := 0
		testable.Env.Getwd = func() (string, error) {
			getwdCalls++
			return "/home/sbs/src/github.com/bassosimone/npte", nil
		}

		require.NoError(t, runMain(context.Background(),
			[]string{"--dry-run", "--sandbox", "client", "ip", "addr"}))

		assert.Equal(t, -1, s.ExitCode)
		assert.Equal(t, 1, getwdCalls, "Getwd must be called once when --sandbox is set")
	})
}

// TestRun_GetwdErrorReachesLogFatal pins that a Getwd failure does not get
// silently dropped — it must reach LogFatalOnError0. Without this guard a
// future refactor could lose the error and continue with workDir="", which
// would emit a malformed bwrap argv (--bind ""  "" --chdir "").
func TestRun_GetwdErrorReachesLogFatal(t *testing.T) {
	s := testenv.Setup(t)
	s.SudoUser = "alice"
	want := errors.New("getwd boom")
	testable.Env.Getwd = func() (string, error) { return "", want }
	// LogFatalOnError0 is also invoked (with nil) by other layers in dry-run
	// mode, so collect every non-nil call rather than capturing the last one.
	var fatals []error
	testable.Env.LogFatalOnError0 = func(err error) {
		if err != nil {
			fatals = append(fatals, err)
		}
	}

	require.NoError(t, runMain(context.Background(),
		[]string{"--dry-run", "--sandbox", "client", "ip", "addr"}))

	assert.Contains(t, fatals, want, "Getwd error must reach LogFatalOnError0")
}

// TestRun_dryRunSkipsLstat pins the contract that dry-run does not consult
// the filesystem to decide whether the namespace is managed.
func TestRun_dryRunSkipsLstat(t *testing.T) {
	s := testenv.Setup(t)
	s.SudoUser = "alice"
	lstatCalls := 0
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		lstatCalls++
		return nil, os.ErrNotExist
	}

	require.NoError(t, runMain(context.Background(),
		[]string{"--dry-run", "client", "ip", "addr"}))

	assert.Equal(t, -1, s.ExitCode, "dry-run must not exit even with no marker on disk")
	assert.Equal(t, 0, lstatCalls, "dry-run must not consult Lstat")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -d -m 0755 /run/npte/netns",
		`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		"ip netns exec client runuser -u alice -- env ip addr",
	})
}

// TestRun_liveRejectsUnmanaged pins the NOPASSWD audit invariant: in live
// mode, run refuses to enter a namespace npte does not own.
func TestRun_liveRejectsUnmanaged(t *testing.T) {
	s := testenv.Setup(t)
	s.SudoUser = "alice"
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	require.NoError(t, runMain(context.Background(),
		[]string{"client", "ip", "addr"}))

	assert.Equal(t, 2, s.ExitCode)
	for _, argv := range s.Commands {
		for _, a := range argv {
			assert.NotEqual(t, "ip", a, "ip must not run when ns is unmanaged")
			assert.NotEqual(t, "runuser", a, "runuser must not run when ns is unmanaged")
		}
	}
}
