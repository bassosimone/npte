// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

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

func TestNetnsList(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.NetnsList(context.Background(), nil, &netnsListInput{})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.True(t, out.Terminated)
		assert.Equal(t, 0, out.ExitCode)

		require.Len(t, s.Commands, 1)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath, "netns", "list",
		}, s.Commands[0])
	})

	t.Run("mkdir fails", func(t *testing.T) {
		testenv.Setup(t)
		errMkdir := errors.New("cannot create proc dir")
		testable.Env.MkdirAll = func(string, os.FileMode) error { return errMkdir }
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.NetnsList(context.Background(), nil, &netnsListInput{})
		assert.ErrorIs(t, err, errMkdir)
		assert.Nil(t, out)
	})
}

func TestNetnsShow(t *testing.T) {
	t.Run("all sections", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &netnsShowInput{Netns: "client"}
		_, out, err := mgr.NetnsShow(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.True(t, out.Terminated)

		require.Len(t, s.Commands, 1)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netns", "show", "--", "client",
		}, s.Commands[0])
	})

	t.Run("specific sections", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &netnsShowInput{
			Netns:    "server",
			Sections: []string{"link", "qdisc"},
		}
		_, out, err := mgr.NetnsShow(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)

		require.Len(t, s.Commands, 1)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netns", "show",
			"--section", "link",
			"--section", "qdisc",
			"--", "server",
		}, s.Commands[0])
	})

	t.Run("mkdir fails", func(t *testing.T) {
		testenv.Setup(t)
		errMkdir := errors.New("cannot create proc dir")
		testable.Env.MkdirAll = func(string, os.FileMode) error { return errMkdir }
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &netnsShowInput{Netns: "client"}
		_, out, err := mgr.NetnsShow(context.Background(), nil, input)
		assert.ErrorIs(t, err, errMkdir)
		assert.Nil(t, out)
	})
}

func TestRunCommand(t *testing.T) {
	t.Run("single run", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &runCommandInput{
			Netns:   "client",
			Argv:    []string{"ping", "-c", "1", "172.16.3.1"},
			Timeout: "10s",
		}
		_, out, err := mgr.RunCommand(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Len(t, out.Steps, 1)
		assert.Equal(t, "run 1", out.Steps[0].Step)
		assert.True(t, out.Steps[0].Terminated)

		require.Len(t, s.Commands, 1)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netns", "run", "--sandbox",
			"--", "client",
			"ping", "-c", "1", "172.16.3.1",
		}, s.Commands[0])
	})

	t.Run("with env and count", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &runCommandInput{
			Netns:   "client",
			Env:     map[string]string{"B_VAR": "two", "A_VAR": "one"},
			Argv:    []string{"curl", "http://172.16.2.1"},
			Timeout: "5s",
			Count:   3,
		}
		_, out, err := mgr.RunCommand(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Len(t, out.Steps, 3)
		assert.Equal(t, "run 1", out.Steps[0].Step)
		assert.Equal(t, "run 2", out.Steps[1].Step)
		assert.Equal(t, "run 3", out.Steps[2].Step)

		require.Len(t, s.Commands, 3)
		expectedArgv := []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netns", "run", "--sandbox",
			"-e", "A_VAR=one",
			"-e", "B_VAR=two",
			"--", "client",
			"curl", "http://172.16.2.1",
		}
		for _, cmd := range s.Commands {
			assert.Equal(t, expectedArgv, cmd)
		}
	})

	t.Run("count zero defaults to one", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &runCommandInput{
			Netns:   "client",
			Argv:    []string{"true"},
			Timeout: "5s",
			Count:   0,
		}
		_, out, err := mgr.RunCommand(context.Background(), nil, input)
		require.NoError(t, err)
		require.Len(t, out.Steps, 1)
		require.Len(t, s.Commands, 1)
	})

	t.Run("invalid timeout", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &runCommandInput{
			Netns:   "client",
			Argv:    []string{"true"},
			Timeout: "not-a-duration",
		}
		_, out, err := mgr.RunCommand(context.Background(), nil, input)
		assert.Error(t, err)
		assert.Nil(t, out)
	})

	t.Run("mkdir fails", func(t *testing.T) {
		testenv.Setup(t)
		errMkdir := errors.New("cannot create proc dir")
		testable.Env.MkdirAll = func(string, os.FileMode) error { return errMkdir }
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &runCommandInput{
			Netns:   "client",
			Argv:    []string{"true"},
			Timeout: "5s",
		}
		_, out, err := mgr.RunCommand(context.Background(), nil, input)
		assert.ErrorIs(t, err, errMkdir)
		assert.Nil(t, out)
	})
}

func TestStartCommand(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &startCommandInput{
			Netns: "server",
			Env:   map[string]string{"PORT": "5201"},
			Argv:  []string{"iperf3", "-s"},
		}
		_, out, err := mgr.StartCommand(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.NotEmpty(t, out.ProcID)
		assert.NotEmpty(t, out.ProcDir)

		require.Len(t, s.Commands, 1)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netns", "run", "--sandbox",
			"-e", "PORT=5201",
			"--", "server",
			"iperf3", "-s",
		}, s.Commands[0])
	})

	t.Run("mkdir fails", func(t *testing.T) {
		testenv.Setup(t)
		errMkdir := errors.New("cannot create proc dir")
		testable.Env.MkdirAll = func(string, os.FileMode) error { return errMkdir }
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &startCommandInput{
			Netns: "server",
			Argv:  []string{"iperf3", "-s"},
		}
		_, out, err := mgr.StartCommand(context.Background(), nil, input)
		assert.ErrorIs(t, err, errMkdir)
		assert.Nil(t, out)
	})
}
