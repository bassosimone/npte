// SPDX-License-Identifier: GPL-3.0-or-later

package subprocess

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipeline_Dry(t *testing.T) {
	var captured error
	var stdout bytes.Buffer
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = &stdout
	env.Stderr = io.Discard
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	MustPipeline(context.Background(), true,
		[]string{"iptables-save"},
		[]string{"grep", "-Fv", "npte:gw:router"},
		[]string{"iptables-restore"},
	)
	assert.NoError(t, captured)
	assert.Equal(t, "iptables-save | grep -Fv npte:gw:router | iptables-restore\n", stdout.String())
}

func TestPipeline_Live(t *testing.T) {
	var captured error
	var stdout bytes.Buffer
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = &stdout
	env.Stderr = io.Discard
	env.LookPath = func(name string) (string, error) {
		switch name {
		case "iptables-save":
			return "/bin/echo", nil
		case "grep":
			return "/bin/grep", nil
		case "iptables-restore":
			return "/bin/cat", nil
		default:
			return "", fmt.Errorf("unexpected: %s", name)
		}
	}
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	// echo "hello" | grep -F hello | cat → "hello\n"
	MustPipeline(context.Background(), false,
		[]string{"iptables-save", "hello"},
		[]string{"grep", "-F", "hello"},
		[]string{"iptables-restore"},
	)
	assert.NoError(t, captured)
	assert.Equal(t, "hello\n", stdout.String())
}

func TestPipeline_Live_Filtering(t *testing.T) {
	var captured error
	var stdout bytes.Buffer
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = &stdout
	env.Stderr = io.Discard
	env.LookPath = func(name string) (string, error) {
		switch name {
		case "iptables-save":
			return "/usr/bin/printf", nil
		case "grep":
			return "/bin/grep", nil
		case "iptables-restore":
			return "/bin/cat", nil
		default:
			return "", fmt.Errorf("unexpected: %s", name)
		}
	}
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	// printf "keep\nremove-tag\nalso-keep\n" | grep -Fv "remove-tag" | cat
	MustPipeline(context.Background(), false,
		[]string{"iptables-save", "keep\nremove-tag\nalso-keep\n"},
		[]string{"grep", "-Fv", "remove-tag"},
		[]string{"iptables-restore"},
	)
	assert.NoError(t, captured)
	assert.Equal(t, "keep\nalso-keep\n", stdout.String())
}

func TestPipeline_NoStages(t *testing.T) {
	err := Pipeline(context.Background(), false)
	assert.ErrorContains(t, err, "pipeline has no stages")
}

// TestPipeline_Live_StartFailureCleanup covers the cleanup path that
// runs when one stage's Start() succeeds but a later stage's Start()
// fails. Stage 0 resolves to /bin/true (forks + execs cleanly, exits
// fast); stage 1 resolves to a path that does not exist, so execve
// returns ENOENT and Start() surfaces it. The wrapper must:
//
//   - close every io.Pipe in the closepool (so no goroutine wedges on
//     a half-open pipe),
//   - Kill+Wait every previously-started stage (here stage 0, which
//     by then is most likely a zombie — Kill on a zombie is a defined
//     no-op and Wait reaps it; both return values are discarded),
//   - return a wrapped error naming the failing stage index and argv0.
func TestPipeline_Live_StartFailureCleanup(t *testing.T) {
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = io.Discard
	env.Stderr = io.Discard
	env.LookPath = func(name string) (string, error) {
		switch name {
		case "iptables-save":
			return "/bin/true", nil
		case "grep":
			// A path that exists nowhere on the FS forces execve(2) to
			// return ENOENT inside cmd.Start(). POSIX-stable; we don't
			// rely on a Linux-specific quirk.
			return "/npte-test-nonexistent/bogus-binary", nil
		default:
			return "", fmt.Errorf("unexpected: %s", name)
		}
	}
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	err := Pipeline(context.Background(), false,
		[]string{"iptables-save"},
		[]string{"grep", "x"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline stage 1")
	assert.Contains(t, err.Error(), "grep")
	assert.Contains(t, err.Error(), "start:")
}

// TestPipeline_Live_LookPathError pins the deps-allowlist gate inside
// the stage setup loop: a non-allowlisted argv0 must surface as an
// allowlist error before any exec.Cmd is built or Start is called, so
// no stub on testable.Env.LookPath is needed (deps.LookPath
// short-circuits on the allowlist check).
func TestPipeline_Live_LookPathError(t *testing.T) {
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = io.Discard
	env.Stderr = io.Discard
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	err := Pipeline(context.Background(), false,
		[]string{"nonexistent-bogus-cmd"},
	)
	assert.ErrorContains(t, err, `command "nonexistent-bogus-cmd" is not in the allowlist`)
}
