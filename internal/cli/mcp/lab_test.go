// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabCreate(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.MkdirAll = os.MkdirAll
	testable.Env.OpenFile = os.OpenFile
	mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

	_, out, err := mgr.LabCreate(context.Background(), nil, &labCreateInput{})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.Terminated)
	assert.Equal(t, 0, out.ExitCode)

	require.Len(t, s.Commands, 1)
	assert.Equal(t, []string{
		"/usr/bin/sudo", "-n", testenv.SelfPath, "lab", "create",
	}, s.Commands[0])
}

func TestLabDestroy(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.MkdirAll = os.MkdirAll
	testable.Env.OpenFile = os.OpenFile
	mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

	_, out, err := mgr.LabDestroy(context.Background(), nil, &labDestroyInput{})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.Terminated)
	assert.Equal(t, 0, out.ExitCode)

	require.Len(t, s.Commands, 1)
	assert.Equal(t, []string{
		"/usr/bin/sudo", "-n", testenv.SelfPath, "lab", "destroy",
	}, s.Commands[0])
}
