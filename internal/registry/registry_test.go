// SPDX-License-Identifier: GPL-3.0-or-later

package registry

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkerPath(t *testing.T) {
	t.Run("valid name", func(t *testing.T) {
		assert.Equal(t, "/run/npte/netns/client", markerPath("client"))
	})
	t.Run("panics on bad name", func(t *testing.T) {
		assert.Panics(t, func() { markerPath("1bad") })
	})
}

func TestLock(t *testing.T) {
	t.Run("dry-run prints install and skips LockFile", func(t *testing.T) {
		s := testenv.Setup(t)
		lockCalled := false
		testable.Env.LockFile = func(string) (func(), error) {
			lockCalled = true
			return func() {}, nil
		}

		unlock, err := Lock(context.Background(), testable.Env, true)
		require.NoError(t, err)
		assert.False(t, lockCalled, "LockFile must not be invoked in dry-run")
		assert.NotNil(t, unlock)
		unlock()

		testenv.AssertLines(t, s.Stdout.String(), []any{
			"install -d -m 0755 /run/npte/netns",
		})
	})

	t.Run("live mode locks the file", func(t *testing.T) {
		s := testenv.Setup(t)
		var locked string
		testable.Env.LockFile = func(path string) (func(), error) {
			locked = path
			return func() {}, nil
		}

		unlock, err := Lock(context.Background(), testable.Env, false)
		require.NoError(t, err)
		assert.Equal(t, "/run/npte/.lock", locked)
		assert.NotNil(t, unlock)
		unlock()
		assert.Empty(t, s.Stdout.String(), "live mode does not print")
	})
}

func TestMustLock_returnsUnlock(t *testing.T) {
	testenv.Setup(t)
	unlock := MustLock(context.Background(), testable.Env, true)
	require.NotNil(t, unlock)
	unlock()
}

func TestRegister(t *testing.T) {
	s := testenv.Setup(t)
	require.NoError(t, Register(context.Background(), true, "client"))
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -m 0644 /dev/null /run/npte/netns/client",
	})
}

func TestMustRegister(t *testing.T) {
	s := testenv.Setup(t)
	MustRegister(context.Background(), true, "client")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -m 0644 /dev/null /run/npte/netns/client",
	})
}

func TestUnregister(t *testing.T) {
	s := testenv.Setup(t)
	require.NoError(t, Unregister(context.Background(), true, "client"))
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"rm -f /run/npte/netns/client",
	})
}

func TestMustUnregister(t *testing.T) {
	s := testenv.Setup(t)
	MustUnregister(context.Background(), true, "client")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"rm -f /run/npte/netns/client",
	})
}

func TestRequireManaged(t *testing.T) {
	t.Run("live mode", func(t *testing.T) {
		tests := []struct {
			name      string
			stat      func(string) (os.FileInfo, error)
			wantErr   string // substring; "" means no error
			wantIsErr error  // errors.Is target; nil means none
		}{{
			name: "regular file: managed",
			stat: func(string) (os.FileInfo, error) {
				return fakeStat{regular: true}, nil
			},
			wantErr: "",
		}, {
			name: "missing marker: ErrNotManaged",
			stat: func(string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			wantErr:   "client",
			wantIsErr: ErrNotManaged,
		}, {
			name: "directory marker: rejects",
			stat: func(string) (os.FileInfo, error) {
				return fakeStat{regular: false}, nil
			},
			wantErr: "marker is not a regular file",
		}, {
			name: "other Stat error: wrapped",
			stat: func(string) (os.FileInfo, error) {
				return nil, errors.New("permission denied")
			},
			wantErr: "registry: permission denied",
		}}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				s := testenv.Setup(t)
				testable.Env.Stat = tc.stat
				err := RequireManaged(testable.Env, false, "client")
				assert.Empty(t, s.Stdout.String(), "live mode must not print to stdout")
				if tc.wantErr == "" {
					assert.NoError(t, err)
					return
				}
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				if tc.wantIsErr != nil {
					assert.ErrorIs(t, err, tc.wantIsErr)
				}
			})
		}
	})

	// Dry-run mode emits a paste-into-shell-faithful guard regardless of
	// what the filesystem actually says. The point of the dry-run branch
	// is precisely to not consult Stat — a dry-run after a dry-run
	// `netns create` finds no real marker, and the live behaviour would
	// abort the rest of the rendered script. We assert that the guard
	// is emitted unchanged in three Stat scenarios that the live branch
	// would treat differently.
	t.Run("dry-run mode emits guard regardless of Stat", func(t *testing.T) {
		stats := map[string]func(string) (os.FileInfo, error){
			"missing": func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
			"regular": func(string) (os.FileInfo, error) { return fakeStat{regular: true}, nil },
			"dir":     func(string) (os.FileInfo, error) { return fakeStat{regular: false}, nil },
		}
		want := `test -f "/run/npte/netns/client" || { echo 'npte: client: not managed by npte' >&2; exit 2; }`
		for label, stat := range stats {
			t.Run(label, func(t *testing.T) {
				s := testenv.Setup(t)
				testable.Env.Stat = stat
				err := RequireManaged(testable.Env, true, "client")
				require.NoError(t, err)
				testenv.AssertLines(t, s.Stdout.String(), []any{want})
			})
		}
	})
}

func TestList(t *testing.T) {
	tests := []struct {
		name    string
		readDir func(string) ([]os.DirEntry, error)
		want    []string
		wantErr string
	}{{
		name:    "missing dir returns empty",
		readDir: func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist },
		want:    nil,
	}, {
		name: "sorts and filters",
		readDir: func(string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeEntry{name: "router", regular: true},
				fakeEntry{name: "client", regular: true},
				fakeEntry{name: "1bad", regular: true},    // invalid netns name
				fakeEntry{name: "subdir", regular: false}, // not a regular file
				fakeEntry{name: "server", regular: true},
			}, nil
		},
		want: []string{"client", "router", "server"},
	}, {
		name:    "other error wrapped",
		readDir: func(string) ([]os.DirEntry, error) { return nil, errors.New("boom") },
		wantErr: "registry: boom",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Setup(t)
			testable.Env.ReadDir = tc.readDir
			got, err := List(testable.Env)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// fakeStat is a [os.FileInfo] whose Mode() reports either a regular file or
// a directory, depending on regular.
type fakeStat struct{ regular bool }

func (fakeStat) Name() string { return "" }
func (fakeStat) Size() int64  { return 0 }
func (f fakeStat) Mode() fs.FileMode {
	if f.regular {
		return 0o644
	}
	return fs.ModeDir | 0o755
}
func (fakeStat) ModTime() time.Time { return time.Time{} }
func (f fakeStat) IsDir() bool      { return !f.regular }
func (fakeStat) Sys() any           { return nil }

// fakeEntry is a minimal [os.DirEntry] for List tests.
type fakeEntry struct {
	name    string
	regular bool
}

func (e fakeEntry) Name() string { return e.name }
func (e fakeEntry) IsDir() bool  { return !e.regular }
func (e fakeEntry) Type() fs.FileMode {
	if e.regular {
		return 0
	}
	return fs.ModeDir
}
func (e fakeEntry) Info() (os.FileInfo, error) {
	return fakeStat{regular: e.regular}, nil
}
