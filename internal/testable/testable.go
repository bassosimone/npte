// SPDX-License-Identifier: GPL-3.0-or-later

// Package testable contains code to make npte testable.
package testable

import (
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/bassosimone/deferexit"
	"github.com/charmbracelet/lipgloss"
	"github.com/rogpeppe/go-internal/lockedfile"
)

// Environ abstracts away side effects (filesystem, execution, locking, exit)
// so that commands can be tested without root, namespaces, or real I/O.
type Environ struct {
	Args             []string
	Exit             func(code int)
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
	LogRenderer      *lipgloss.Renderer
	Getenv           func(key string) string
	Geteuid          func() int
	MkdirAll         func(path string, perm os.FileMode) error
	ReadFile         func(name string) ([]byte, error)
	WriteFile        func(name string, data []byte, perm os.FileMode) error
	Lstat            func(name string) (os.FileInfo, error)
	Remove           func(name string) error
	ReadDir          func(name string) ([]os.DirEntry, error)
	RunCommand       func(cmd *exec.Cmd) error
	LookPath         func(file string) (string, error)
	Executable       func() (string, error)
	LockFile         func(path string) (func(), error)
	LogFatalOnError0 func(err error)
}

// NewEnvironOS returns an [*Environ] wired to real OS operations.
func NewEnvironOS() *Environ {
	return &Environ{
		Args:        os.Args,
		Exit:        deferexit.Panic,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		LogRenderer: lipgloss.NewRenderer(os.Stderr),
		Getenv:      os.Getenv,
		Geteuid:     os.Geteuid,
		MkdirAll:    os.MkdirAll,
		ReadFile:    os.ReadFile,
		WriteFile:   os.WriteFile,
		Lstat:       os.Lstat,
		Remove:      os.Remove,
		ReadDir:     os.ReadDir,
		RunCommand: func(cmd *exec.Cmd) error {
			return cmd.Run()
		},
		LookPath:   exec.LookPath,
		Executable: os.Executable,
		LockFile: func(path string) (func(), error) {
			return lockedfile.MutexAt(path).Lock()
		},
		LogFatalOnError0: func(err error) {
			if err != nil {
				log.Print(err)
				deferexit.Panic(1)
			}
		},
	}
}

// Env is the global [*Environ].
var Env = NewEnvironOS()
