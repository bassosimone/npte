// SPDX-License-Identifier: GPL-3.0-or-later

package exitx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Recover converts a Do panic into a call to onExit with the same code.
func TestRecover_CatchesExitPanic(t *testing.T) {
	var got int
	called := false
	func() {
		defer Recover(func(code int) {
			got = code
			called = true
		})
		Do(7)
		t.Fatal("Do must not return")
	}()
	assert.True(t, called, "onExit must be called")
	assert.Equal(t, 7, got)
}

// Recover re-raises non-Exit panics so real bugs are not swallowed.
func TestRecover_ReraisesOtherPanics(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "non-Exit panic must propagate")
		assert.Equal(t, "boom", r)
	}()
	defer Recover(func(int) {
		t.Fatal("onExit must not be called for non-Exit panics")
	})
	panic("boom")
}

// Recover with no panic in flight is a no-op.
func TestRecover_NoPanic(t *testing.T) {
	defer Recover(func(int) {
		t.Fatal("onExit must not be called when there is no panic")
	})
}
