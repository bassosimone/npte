// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWait(t *testing.T) {
	t.Run("invalid timeout", func(t *testing.T) {
		testenv.Setup(t)
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &waitInput{ProcID: "irrelevant", Timeout: "bad"}
		_, out, err := mgr.Wait(context.Background(), nil, input)
		assert.Error(t, err)
		assert.Nil(t, out)
	})

	t.Run("unknown proc ID", func(t *testing.T) {
		testenv.Setup(t)
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &waitInput{ProcID: "nonexistent", Timeout: "1s"}
		_, out, err := mgr.Wait(context.Background(), nil, input)
		assert.ErrorContains(t, err, "no such process")
		assert.Nil(t, out)
	})

	t.Run("process already terminated", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		proc, err := mgr.startProc([]string{"netns", "list"})
		require.NoError(t, err)
		<-proc.done

		input := &waitInput{ProcID: proc.procID, Timeout: "1s"}
		_, out, err := mgr.Wait(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.True(t, out.Terminated)
		assert.Equal(t, 0, out.ExitCode)
	})

	t.Run("timeout before termination", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		unblock := make(chan struct{})
		testable.Env.WaitCommand = func(cmd *exec.Cmd) error {
			<-unblock
			return nil
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		proc, err := mgr.startProc([]string{"netns", "list"})
		require.NoError(t, err)

		input := &waitInput{ProcID: proc.procID, Timeout: "10ms"}
		_, out, err := mgr.Wait(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.False(t, out.Terminated)

		close(unblock)
		<-proc.done
	})
}

func TestKill(t *testing.T) {
	t.Run("unsupported signal", func(t *testing.T) {
		testenv.Setup(t)
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &killInput{ProcID: "irrelevant", Signal: "KILL"}
		_, out, err := mgr.Kill(context.Background(), nil, input)
		assert.ErrorContains(t, err, "unsupported signal")
		assert.Nil(t, out)
	})

	t.Run("unknown proc ID", func(t *testing.T) {
		testenv.Setup(t)
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		input := &killInput{ProcID: "nonexistent", Signal: "INT"}
		_, out, err := mgr.Kill(context.Background(), nil, input)
		assert.ErrorContains(t, err, "no such process")
		assert.Nil(t, out)
	})

	t.Run("signal with SIG prefix", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		unblock := make(chan struct{})
		testable.Env.WaitCommand = func(cmd *exec.Cmd) error {
			<-unblock
			return nil
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		proc, err := mgr.startProc([]string{"netns", "run", "--sandbox", "--", "client", "iperf3", "-s"})
		require.NoError(t, err)

		input := &killInput{ProcID: proc.procID, Signal: "SIGINT"}
		_, out, err := mgr.Kill(context.Background(), nil, input)
		require.NoError(t, err)
		require.NotNil(t, out)

		close(unblock)
		<-proc.done
	})

	t.Run("kill already terminated process", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		proc, err := mgr.startProc([]string{"netns", "list"})
		require.NoError(t, err)
		<-proc.done

		input := &killInput{ProcID: proc.procID, Signal: "INT"}
		_, _, err = mgr.Kill(context.Background(), nil, input)
		assert.ErrorIs(t, err, os.ErrProcessDone)
	})
}

func TestCleanup(t *testing.T) {
	t.Run("no running processes", func(t *testing.T) {
		testenv.Setup(t)
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		err := mgr.Cleanup(context.Background())
		assert.NoError(t, err)
	})

	t.Run("processes terminate promptly", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		unblock := make(chan struct{})
		testable.Env.WaitCommand = func(cmd *exec.Cmd) error {
			<-unblock
			return nil
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, err := mgr.startProc([]string{"netns", "run", "--sandbox", "--", "client", "sleep", "999"})
		require.NoError(t, err)
		_, err = mgr.startProc([]string{"netns", "run", "--sandbox", "--", "server", "sleep", "999"})
		require.NoError(t, err)

		close(unblock)
		err = mgr.Cleanup(context.Background())
		assert.NoError(t, err)
	})

	t.Run("context expires before termination", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		testable.Env.OpenFile = os.OpenFile
		unblock := make(chan struct{})
		testable.Env.WaitCommand = func(cmd *exec.Cmd) error {
			<-unblock
			return nil
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		_, err := mgr.startProc([]string{"netns", "run", "--sandbox", "--", "client", "sleep", "999"})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		err = mgr.Cleanup(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)

		close(unblock)
	})
}

func TestRunProc_timeoutKillsProcess(t *testing.T) {
	testenv.Setup(t)
	testable.Env.MkdirAll = os.MkdirAll
	testable.Env.OpenFile = os.OpenFile
	unblock := make(chan struct{})
	testable.Env.WaitCommand = func(cmd *exec.Cmd) error {
		<-unblock
		return nil
	}
	mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

	// Use a very short timeout so runProc hits the !wait.Terminated branch
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(unblock)
	}()
	out, err := mgr.runProc(context.Background(), 10*time.Millisecond, []string{"netns", "list"})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.Terminated)
}

func TestStartProc_openFileFailures(t *testing.T) {
	t.Run("exitcode.txt open fails", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		errOpen := errors.New("cannot open exitcode.txt")
		testable.Env.OpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			return nil, errOpen
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		proc, err := mgr.startProc([]string{"netns", "list"})
		assert.ErrorIs(t, err, errOpen)
		assert.Nil(t, proc)
	})

	t.Run("stdout.txt open fails", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		errOpen := errors.New("cannot open stdout.txt")
		calls := 0
		testable.Env.OpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			calls++
			if calls > 1 {
				return nil, errOpen
			}
			return os.OpenFile(name, flag, perm)
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		proc, err := mgr.startProc([]string{"netns", "list"})
		assert.ErrorIs(t, err, errOpen)
		assert.Nil(t, proc)
	})

	t.Run("stderr.txt open fails", func(t *testing.T) {
		testenv.Setup(t)
		testable.Env.MkdirAll = os.MkdirAll
		errOpen := errors.New("cannot open stderr.txt")
		calls := 0
		testable.Env.OpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			calls++
			if calls > 2 {
				return nil, errOpen
			}
			return os.OpenFile(name, flag, perm)
		}
		mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

		proc, err := mgr.startProc([]string{"netns", "list"})
		assert.ErrorIs(t, err, errOpen)
		assert.Nil(t, proc)
	})
}

func TestStartProc_writeFileFails(t *testing.T) {
	testenv.Setup(t)
	testable.Env.MkdirAll = os.MkdirAll
	testable.Env.OpenFile = os.OpenFile
	errWrite := errors.New("cannot write argv.json")
	testable.Env.WriteFile = func(string, []byte, os.FileMode) error {
		return errWrite
	}
	mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

	proc, err := mgr.startProc([]string{"netns", "list"})
	assert.ErrorIs(t, err, errWrite)
	assert.Nil(t, proc)
}

func TestReaper_exitError(t *testing.T) {
	testenv.Setup(t)
	testable.Env.MkdirAll = os.MkdirAll
	testable.Env.OpenFile = os.OpenFile
	// Run a real command that exits non-zero to get a genuine ExitError
	testable.Env.StartCommand = func(cmd *exec.Cmd) error { return nil }
	testable.Env.WaitCommand = func(cmd *exec.Cmd) error {
		// Run "false" to get a real *exec.ExitError with exit code 1
		c := exec.Command("false")
		return c.Run()
	}
	mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

	proc, err := mgr.startProc([]string{"netns", "list"})
	require.NoError(t, err)
	<-proc.done
	assert.Equal(t, 1, proc.exitcode)
}

func TestReaper_nonExitError(t *testing.T) {
	testenv.Setup(t)
	testable.Env.MkdirAll = os.MkdirAll
	testable.Env.OpenFile = os.OpenFile
	testable.Env.WaitCommand = func(cmd *exec.Cmd) error {
		return errors.New("unexpected wait error")
	}
	mgr := newSessionManager(testable.Env, testenv.SelfPath, t.TempDir())

	proc, err := mgr.startProc([]string{"netns", "list"})
	require.NoError(t, err)
	<-proc.done
	assert.Equal(t, 255, proc.exitcode)
}
