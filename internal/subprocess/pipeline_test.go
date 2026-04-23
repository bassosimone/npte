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
