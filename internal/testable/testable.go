// SPDX-License-Identifier: GPL-3.0-or-later

// Package testable contains code to make npte testable.
package testable

import (
	"io"
	"os"
	"os/exec"

	"github.com/bassosimone/runtimex"
	"github.com/charmbracelet/lipgloss"
	"github.com/rogpeppe/go-internal/lockedfile"
)

// Environ abstracts away side effects (filesystem, execution, locking, exit)
// so that commands can be tested without root, namespaces, or real I/O.
type Environ struct {
	Exit             func(code int)
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
	LogRenderer      *lipgloss.Renderer
	MkdirAll         func(path string, perm os.FileMode) error
	ReadFile         func(name string) ([]byte, error)
	WriteFile        func(name string, data []byte, perm os.FileMode) error
	Stat             func(name string) (os.FileInfo, error)
	Remove           func(name string) error
	RunCommand       func(cmd *exec.Cmd) error
	LookPath         func(file string) (string, error)
	LockFile         func(path string) (func(), error)
	LogFatalOnError0 func(err error)
}

// NewEnvironOS returns an [*Environ] wired to real OS operations.
func NewEnvironOS() *Environ {
	return &Environ{
		Exit:        os.Exit,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		LogRenderer: lipgloss.NewRenderer(os.Stderr),
		MkdirAll:    os.MkdirAll,
		ReadFile:    os.ReadFile,
		WriteFile:   os.WriteFile,
		Stat:        os.Stat,
		Remove:      os.Remove,
		RunCommand: func(cmd *exec.Cmd) error {
			return cmd.Run()
		},
		LookPath: exec.LookPath,
		LockFile: func(path string) (func(), error) {
			return lockedfile.MutexAt(path).Lock()
		},
		LogFatalOnError0: runtimex.LogFatalOnError0,
	}
}

// Env is the global [*Environ].
var Env = NewEnvironOS()
