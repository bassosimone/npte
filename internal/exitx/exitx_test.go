// SPDX-License-Identifier: GPL-3.0-or-later

package exitx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Recover converts a Panic panic into a call to onExit with the same code.
func TestRecover_CatchesExitPanic(t *testing.T) {
	var got int
	called := false
	func() {
		defer Recover(func(code int) {
			got = code
			called = true
		})
		Panic(7)
		t.Fatal("Panic must not return")
	}()
	assert.True(t, called, "onExit must be called")
	assert.Equal(t, 7, got)
}

// Run converts a Panic panic into the corresponding exit code.
func TestRun_CatchesExitPanic(t *testing.T) {
	code := Run(func() {
		Panic(7)
		t.Fatal("Panic must not return")
	})
	assert.Equal(t, 7, code)
}

// Recover re-raises non-Code panics so real bugs are not swallowed.
func TestRecover_ReraisesOtherPanics(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "non-Code panic must propagate")
		assert.Equal(t, "boom", r)
	}()
	defer Recover(func(int) {
		t.Fatal("onExit must not be called for non-Code panics")
	})
	panic("boom")
}

// Run re-raises non-Code panics so real bugs are not swallowed.
func TestRun_ReraisesOtherPanics(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "non-Code panic must propagate")
		assert.Equal(t, "boom", r)
	}()
	_ = Run(func() {
		panic("boom")
	})
}

// A raw int panic must NOT be intercepted as an exit code: [Code]
// is a named type distinct from int, and only deliberate [Panic]
// calls (which wrap the value in a [Code]) participate in the exit
// protocol.
func TestRecover_RawIntPanicPropagates(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "raw int panic must propagate")
		v, ok := r.(int)
		assert.True(t, ok, "panic value must keep its int type, not be converted to Code")
		assert.Equal(t, 4, v)
	}()
	defer Recover(func(int) {
		t.Fatal("onExit must NOT be called for a raw int panic")
	})
	panic(4)
}

// Same as above for Run.
func TestRun_RawIntPanicPropagates(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "raw int panic must propagate")
		v, ok := r.(int)
		assert.True(t, ok, "panic value must keep its int type, not be converted to Code")
		assert.Equal(t, 4, v)
	}()
	_ = Run(func() { panic(4) })
	t.Fatal("Run must not swallow a raw int panic")
}

// Recover with no panic in flight is a no-op.
func TestRecover_NoPanic(t *testing.T) {
	defer Recover(func(int) {
		t.Fatal("onExit must not be called when there is no panic")
	})
}

// Run with no panic in flight returns zero.
func TestRun_NoPanic(t *testing.T) {
	code := Run(func() { /* nothing */ })
	assert.Equal(t, code, 0)
}
