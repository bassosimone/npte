// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShapeDownload(t *testing.T) {
	t.Run("happy path with all fields", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &shapeInput{
			Delay:         "10ms",
			Loss:          "1%",
			Limit:         "1000",
			Rate:          "10mbit",
			Slot:          "5ms 10ms",
			Child:         "cake",
			CakeBandwidth: "30mbit",
		}
		_, out, err := mgr.ShapeDownload(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Len(t, out.Steps, 2)
		assert.Equal(t, "clear", out.Steps[0].Step)
		assert.True(t, out.Steps[0].Terminated)
		assert.Equal(t, "apply", out.Steps[1].Step)
		assert.True(t, out.Steps[1].Terminated)

		require.Len(t, s.Commands, 2)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netem", "clear", "--", "router", "if-client",
		}, s.Commands[0])
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netem", "apply",
			"--delay", "10ms",
			"--loss", "1%",
			"--limit", "1000",
			"--rate", "10mbit",
			"--slot", "5ms 10ms",
			"--child", "cake",
			"--cake-bandwidth", "30mbit",
			"--", "router", "if-client",
		}, s.Commands[1])
	})

	t.Run("only rate set", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &shapeInput{Rate: "5mbit"}
		_, out, err := mgr.ShapeDownload(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)

		require.Len(t, s.Commands, 2)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netem", "apply", "--rate", "5mbit",
			"--", "router", "if-client",
		}, s.Commands[1])
	})

	t.Run("clear command fails to start", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		errClear := errors.New("simulated clear failure")
		testable.Env.StartCommand = func(cmd *exec.Cmd) error {
			s.Commands = append(s.Commands, append([]string{}, cmd.Args...))
			if slices.Contains(cmd.Args, "clear") {
				return errClear
			}
			return nil
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeDownload(context.Background(), nil, &shapeInput{Rate: "10mbit"})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, 127, out.Steps[0].ExitCode)
		assert.True(t, out.Steps[0].Terminated)

		require.Len(t, s.Commands, 2)
	})

	t.Run("apply command fails to start", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		errApply := errors.New("simulated apply failure")
		testable.Env.StartCommand = func(cmd *exec.Cmd) error {
			s.Commands = append(s.Commands, append([]string{}, cmd.Args...))
			if slices.Contains(cmd.Args, "apply") {
				return errApply
			}
			return nil
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeDownload(context.Background(), nil, &shapeInput{Rate: "10mbit"})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, 0, out.Steps[0].ExitCode)
		assert.Equal(t, 127, out.Steps[1].ExitCode)
		assert.True(t, out.Steps[1].Terminated)

		require.Len(t, s.Commands, 2)
	})

	t.Run("clear mkdir fails", func(t *testing.T) {
		testenv.Setup(t)
		errInfra := errors.New("cannot create proc dir")
		testable.Env.MkdirAll = func(string, os.FileMode) error { return errInfra }
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeDownload(context.Background(), nil, &shapeInput{Rate: "10mbit"})
		assert.ErrorIs(t, err, errInfra)
		assert.Nil(t, out)
	})

	t.Run("apply mkdir fails", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.OpenFile = os.OpenFile
		errInfra := errors.New("cannot create proc dir")
		calls := 0
		testable.Env.MkdirAll = func(path string, perm os.FileMode) error {
			calls++
			if calls > 1 {
				return errInfra
			}
			return os.MkdirAll(path, perm)
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeDownload(context.Background(), nil, &shapeInput{Rate: "10mbit"})
		assert.ErrorIs(t, err, errInfra)
		assert.Nil(t, out)
	})
}

func TestShapeUpload(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &shapeInput{Delay: "20ms", Rate: "50mbit"}
		_, out, err := mgr.ShapeUpload(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Len(t, out.Steps, 2)

		require.Len(t, s.Commands, 2)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netem", "clear", "--", "router", "if-server",
		}, s.Commands[0])
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netem", "apply",
			"--delay", "20ms",
			"--rate", "50mbit",
			"--", "router", "if-server",
		}, s.Commands[1])
	})
}

func TestShapeClear(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeClear(context.Background(), nil, &shapeClearInput{})
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Len(t, out.Steps, 2)
		assert.Equal(t, "clear download", out.Steps[0].Step)
		assert.Equal(t, "clear upload", out.Steps[1].Step)

		require.Len(t, s.Commands, 2)
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netem", "clear", "--", "router", "if-client",
		}, s.Commands[0])
		assert.Equal(t, []string{
			"/usr/bin/sudo", "-n", testenv.SelfPath,
			"netem", "clear", "--", "router", "if-server",
		}, s.Commands[1])
	})

	t.Run("first clear fails to start", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		errFirst := errors.New("simulated first clear failure")
		testable.Env.StartCommand = func(cmd *exec.Cmd) error {
			s.Commands = append(s.Commands, append([]string{}, cmd.Args...))
			return errFirst
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeClear(context.Background(), nil, &shapeClearInput{})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, 127, out.Steps[0].ExitCode)
		assert.True(t, out.Steps[0].Terminated)

		require.Len(t, s.Commands, 2)
	})

	t.Run("second clear fails to start", func(t *testing.T) {
		s := testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		errSecond := errors.New("simulated second clear failure")
		testable.Env.StartCommand = func(cmd *exec.Cmd) error {
			s.Commands = append(s.Commands, append([]string{}, cmd.Args...))
			if slices.Contains(cmd.Args, "if-server") {
				return errSecond
			}
			return nil
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeClear(context.Background(), nil, &shapeClearInput{})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, 0, out.Steps[0].ExitCode)
		assert.Equal(t, 127, out.Steps[1].ExitCode)

		require.Len(t, s.Commands, 2)
	})

	t.Run("first clear mkdir fails", func(t *testing.T) {
		testenv.Setup(t)
		errInfra := errors.New("cannot create proc dir")
		testable.Env.MkdirAll = func(string, os.FileMode) error { return errInfra }
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeClear(context.Background(), nil, &shapeClearInput{})
		assert.ErrorIs(t, err, errInfra)
		assert.Nil(t, out)
	})

	t.Run("second clear mkdir fails", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.OpenFile = os.OpenFile
		errInfra := errors.New("cannot create proc dir")
		calls := 0
		testable.Env.MkdirAll = func(path string, perm os.FileMode) error {
			calls++
			if calls > 1 {
				return errInfra
			}
			return os.MkdirAll(path, perm)
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, out, err := mgr.ShapeClear(context.Background(), nil, &shapeClearInput{})
		assert.ErrorIs(t, err, errInfra)
		assert.Nil(t, out)
	})
}
