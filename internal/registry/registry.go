// SPDX-License-Identifier: GPL-3.0-or-later

// Package registry tracks the network namespaces that npte manages.
//
// A netns is "managed by npte" when an empty marker file exists at
// /run/npte/netns/<name>. Markers are created at the end of `netns
// create` and removed at the end of `netns destroy`. Every other verb
// that touches a named netns calls [RequireManaged] before doing
// anything, so the privileged surface exposed via the NOPASSWD
// sudoers grant is bounded to the namespaces npte itself created.
//
// Concurrency: every state-touching verb takes the global lock via
// [Lock] at the top of its run, after shape validators and before any
// kernel or marker operation. The lock serializes all npte
// invocations so that "kernel op then marker op" is observable as
// atomic to other npte processes.
package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
)

const (
	netnsDir = "/run/npte/netns"
	lockPath = "/run/npte/.lock"
)

// ErrNotManaged is the sentinel returned by [RequireManaged] when a
// name has no marker. Callers can match it with errors.Is.
var ErrNotManaged = errors.New("namespace not managed by npte")

// markerPath returns the absolute marker path for name. It panics if
// name does not pass [validate.NetnsName]: callers (the cli verbs)
// already validate user-supplied names against that same predicate
// before calling into this package, so a panic here means an internal
// caller passed an unvalidated value. Failing loud is the right
// response — never construct a marker path from untrusted bytes.
func markerPath(name string) string {
	runtimex.PanicOnError0(validate.NetnsName(name))
	return filepath.Join(netnsDir, name)
}

// Lock acquires the global npte state lock and ensures
// /run/npte/netns/ exists with mode 0755. Callers MUST defer the
// returned unlock function.
//
// In dry-run mode, the install command is printed to env.Stdout and
// the kernel lock is not taken (dry-run is a side-effect-free preview
// and does not need to be serialized against concurrent npte
// invocations). The returned unlock function is a no-op in that case.
func Lock(ctx context.Context, env *testable.Environ, dryRun bool) (func(), error) {
	if err := subprocess.Run(ctx, dryRun, "install", "-d", "-m", "0755", netnsDir); err != nil {
		return nil, err
	}
	if dryRun {
		return func() {}, nil
	}
	return env.LockFile(lockPath)
}

// MustLock is like [Lock] but logs and exits on failure. Returns the
// unlock function, which the caller MUST defer.
func MustLock(ctx context.Context, env *testable.Environ, dryRun bool) func() {
	unlock, err := Lock(ctx, env, dryRun)
	env.LogFatalOnError0(err)
	return unlock
}

// Register creates the marker for name. With dryRun=true, the
// equivalent shell command is printed on env.Stdout instead of being
// executed.
//
// The directory /run/npte/netns/ is created by [Lock]; Register only
// writes the marker file. Callers MUST validate name (e.g. via
// [validate.NetnsName]) and hold the global lock before calling.
func Register(ctx context.Context, dryRun bool, name string) error {
	return subprocess.Run(ctx, dryRun, "install", "-m", "0644", "/dev/null", markerPath(name))
}

// MustRegister is like [Register] but logs and exits on failure.
func MustRegister(ctx context.Context, dryRun bool, name string) {
	testable.Env.LogFatalOnError0(Register(ctx, dryRun, name))
}

// Unregister removes the marker for name. With dryRun=true, the
// equivalent shell command is printed on env.Stdout instead of being
// executed.
//
// Callers MUST validate name (e.g. via [validate.NetnsName]) and hold
// the global lock (see [Lock]) before calling.
func Unregister(ctx context.Context, dryRun bool, name string) error {
	return subprocess.Run(ctx, dryRun, "rm", "-f", markerPath(name))
}

// MustUnregister is like [Unregister] but logs and exits on failure.
func MustUnregister(ctx context.Context, dryRun bool, name string) {
	testable.Env.LogFatalOnError0(Unregister(ctx, dryRun, name))
}

// RequireManaged returns nil if a regular-file marker exists for
// name, [ErrNotManaged] (wrapped with the name) if no marker is
// present, or another error if the marker path resolves to something
// other than a regular file (which should never arise in practice and
// is worth surfacing rather than silently treating as "not managed").
//
// Callers MUST validate name (e.g. via [validate.NetnsName]) and hold
// the global lock (see [Lock]) before calling.
func RequireManaged(env *testable.Environ, name string) error {
	info, err := env.Stat(markerPath(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotManaged, name)
		}
		return fmt.Errorf("registry: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("registry: %s: marker is not a regular file", markerPath(name))
	}
	return nil
}

// List returns the sorted names of all regular-file markers under
// /run/npte/netns/ whose name passes [validate.NetnsName]. If the
// directory does not yet exist (no netns has ever been registered on
// this host), List returns an empty slice and no error. Non-regular
// entries (directories, symlinks) and entries with malformed names
// are silently skipped; the latter would otherwise flow into
// downstream registry calls and trip [markerPath]'s panic guard.
//
// Callers MUST hold the global lock (see [Lock]).
func List(env *testable.Environ) ([]string, error) {
	entries, err := env.ReadDir(netnsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("registry: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if validate.NetnsName(e.Name()) != nil {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}
