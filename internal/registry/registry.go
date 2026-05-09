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
//
// Crash recovery: the lock is dropped when the process dies, but
// markers persist on tmpfs across the lifetime of the running
// system (reboot wipes both /run and all kernel netns, so a fresh
// boot is always a clean slate). If a verb is killed mid-sequence
// without a reboot — SIGKILL from OOM or `kill -9`, an unrecovered
// panic — the registry can end up with an orphan marker (destroy
// crashed before unlink) or an orphan kernel netns (create crashed
// before register). Recovery is by the operator's hands: orphan
// markers clear with `sudo rm /run/npte/netns/<name>`, orphan
// kernel netns clear with `sudo ip netns del <name>`. We
// deliberately do not ship a `gc` verb: a marker carries no
// metadata linking it to a specific kernel-side identity, so an
// automated reconciler cannot distinguish a stale orphan marker
// from a stale marker plus a same-named foreign netns created
// out-of-band, and would risk silently bringing the privileged
// surface to bear on a namespace npte did not create.
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
	"github.com/kballard/go-shellquote"
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

// RequireManaged enforces the ownership predicate for name.
//
// In live mode it returns nil if a regular-file marker exists for name,
// [ErrNotManaged] (wrapped with the name) if no marker is present, or
// another error if the marker dirent is not a regular file (which
// should never arise in practice and is worth surfacing rather than
// silently treating as "not managed").
//
// We use [Lstat] rather than Stat so that a symlink at the marker path
// is detected as non-regular regardless of what it points to. Under
// the documented trust model (root-owned 0755 /run/npte/netns/, only
// npte writes into it) a symlink there cannot arise from an attacker;
// this is defense-in-depth and also keeps RequireManaged consistent
// with [List], which uses dirent-level type information and already
// skips non-regular entries.
//
// In dry-run mode it does NOT Stat the filesystem: a dry-run after a
// dry-run `netns create` would otherwise spuriously fail because no
// real marker was written. Instead it emits a POSIX shell guard on
// env.Stdout — `test -f "<marker>" || { echo ... >&2; exit 2; }` —
// so the rendered script is paste-into-shell faithful: when run as one
// piece it succeeds (the prior `netns create` lines write the marker),
// and when run against a topology that does not exist it fails at the
// same point a live invocation would.
//
// Callers MUST validate name (e.g. via [validate.NetnsName]) and hold
// the global lock (see [Lock]) before calling.
func RequireManaged(env *testable.Environ, dryRun bool, name string) error {
	mp := markerPath(name)
	if dryRun {
		// The shell guard uses `test -f`, which follows symlinks: the
		// live branch's Lstat rejects a symlink at the marker path,
		// but the rendered script accepts a symlink that resolves to
		// a regular file. This residual asymmetry is intentional —
		// the trust model (root-owned 0755 /run/npte/netns/) already
		// excludes a symlink at the marker path, and keeping the
		// guard a single `test -f` preserves diagnostic clarity in
		// the rendered script. Do not "fix" it by switching to
		// `{ test -f X && test ! -L X; }` without weighing that.
		_, err := fmt.Fprintf(env.Stdout, "test -f %s || { echo %s >&2; exit 2; }\n",
			shellquote.Join(mp),
			shellquote.Join(fmt.Sprintf("npte: %s: not managed by npte", name)))
		return err
	}
	info, err := env.Lstat(mp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotManaged, name)
		}
		return fmt.Errorf("registry: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("registry: %s: marker is not a regular file", mp)
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
