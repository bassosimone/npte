// SPDX-License-Identifier: GPL-3.0-or-later

package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetemDelay(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"plain", "10ms", ""},
		{"fractional", "10.5ms", ""},
		{"microsecond", "500us", ""},
		{"second", "1s", ""},
		{"with jitter", "10ms 2ms", ""},
		{"with correlation", "10ms 2ms 25%", ""},
		{"with distribution", "10ms 2ms distribution paretonormal", ""},
		{"distribution without jitter", "10ms distribution uniform", ""},
		{"extra spaces", "  10ms   2ms  ", ""},
		{"tab separator", "10ms\t2ms", ""},
		{"empty", "", "invalid --delay"},
		{"correlation without jitter", "10ms 25%", "invalid --delay"},
		{"missing time", "distribution paretonormal", "invalid --delay"},
		{"unknown distribution", "10ms 1ms distribution evil", "invalid --delay"},
		{"bare number", "10", "invalid --delay"},
		{"missing unit", "10ms 2", "invalid --delay"},
		{"smuggle loss", "10ms loss random 100%", "invalid --delay"},
		{"smuggle ecn", "10ms ecn", "invalid --delay"},
		{"smuggle rate", "10ms rate 1mbit", "invalid --delay"},
		{"smuggle reorder", "reorder 25%", "invalid --delay"},
		{"smuggle corrupt", "corrupt 1%", "invalid --delay"},
		{"smuggle duplicate", "duplicate 1%", "invalid --delay"},
		{"smuggle gap", "gap 10", "invalid --delay"},
		{"smuggle limit", "limit 1000", "invalid --delay"},
		{"shell metachar", "10ms;rm", "invalid --delay"},
		{"trailing junk", "10ms 2ms 25% extra", "invalid --delay"},
		{"distribution with extra", "10ms distribution paretonormal extra", "invalid --delay"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NetemDelay(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNetemLoss(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"bare pct", "1%", ""},
		{"fractional pct", "0.5%", ""},
		{"random pct", "random 1%", ""},
		{"random pct correlation", "random 1% 25%", ""},
		{"random correlation no random kw", "1% 25%", ""},
		{"state min", "state 1%", ""},
		{"state full", "state 1% 2% 3% 4% 5%", ""},
		{"gemodel min", "gemodel 0.1", ""},
		{"gemodel full bare", "gemodel 0.1 0.05 0.9 0.95", ""},
		{"gemodel pct", "gemodel 10% 5%", ""},
		{"extra spaces", "  random   1%  ", ""},
		{"empty", "", "invalid --loss"},
		{"random no pct", "random", "invalid --loss"},
		{"state no pct", "state", "invalid --loss"},
		{"gemodel no prob", "gemodel", "invalid --loss"},
		{"state too many", "state 1% 2% 3% 4% 5% 6%", "invalid --loss"},
		{"gemodel too many", "gemodel 0.1 0.2 0.3 0.4 0.5", "invalid --loss"},
		{"smuggle ecn", "1% ecn", "invalid --loss"},
		{"smuggle delay", "delay 10ms", "invalid --loss"},
		{"smuggle rate", "1% rate 1mbit", "invalid --loss"},
		{"shell metachar", "1%;rm", "invalid --loss"},
		{"unknown model", "weibull 1%", "invalid --loss"},
		{"random no unit", "random 1", "invalid --loss"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NetemLoss(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNetemLimit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"small", "0", ""},
		{"typical", "1000", ""},
		{"large", "1000000", ""},
		{"trimmed", "  1000  ", ""},
		{"empty", "", "invalid --limit"},
		{"negative", "-1", "invalid --limit"},
		{"float", "10.5", "invalid --limit"},
		{"with unit", "1000ms", "invalid --limit"},
		{"two tokens", "1000 2000", "invalid --limit"},
		{"smuggle delay", "delay 10ms", "invalid --limit"},
		{"smuggle ecn", "1000 ecn", "invalid --limit"},
		{"shell metachar", "1000;rm", "invalid --limit"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NetemLimit(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNetemRate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"plain mbit", "10mbit", ""},
		{"plain kbit", "100kbit", ""},
		{"plain gbit", "1gbit", ""},
		{"plain tbit", "1tbit", ""},
		{"plain bit", "1000bit", ""},
		{"fractional rate", "1.5mbit", ""},
		{"with packet overhead", "10mbit 1000", ""},
		{"with negative overhead", "10mbit -14", ""},
		{"with cellsize", "10mbit 1000 500", ""},
		{"with cell overhead", "10mbit 1000 500 -8", ""},
		{"trimmed", "  10mbit  ", ""},
		{"empty", "", "invalid --rate"},
		{"missing unit", "10", "invalid --rate"},
		{"unknown unit", "10gigabit", "invalid --rate"},
		{"cellsize negative", "10mbit 1000 -500", "invalid --rate"},
		{"too many tokens", "10mbit 1000 500 -8 0", "invalid --rate"},
		{"smuggle delay", "10mbit delay 10ms", "invalid --rate"},
		{"smuggle ecn", "10mbit ecn", "invalid --rate"},
		{"shell metachar", "10mbit;rm", "invalid --rate"},
		{"only signed num", "-1000", "invalid --rate"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NetemRate(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNetemSlot(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"min only", "5ms", ""},
		{"min max", "5ms 10ms", ""},
		{"min max packets", "5ms 10ms packets 64", ""},
		{"min max bytes", "5ms 10ms bytes 8192", ""},
		{"min max packets bytes", "5ms 10ms packets 64 bytes 8192", ""},
		{"min packets only", "5ms packets 64", ""},
		{"distribution form", "distribution paretonormal 5ms 1ms", ""},
		{"distribution form with suffixes", "distribution normal 5ms 1ms packets 64 bytes 8192", ""},
		{"trimmed", "  5ms   10ms  ", ""},
		{"empty", "", "invalid --slot"},
		{"no time", "packets 64", "invalid --slot"},
		{"bytes before packets", "5ms 10ms bytes 8192 packets 64", "invalid --slot"},
		{"packets without num", "5ms 10ms packets", "invalid --slot"},
		{"bytes without num", "5ms 10ms bytes", "invalid --slot"},
		{"distribution without times", "distribution normal", "invalid --slot"},
		{"distribution one time", "distribution normal 5ms", "invalid --slot"},
		{"distribution unknown", "distribution evil 5ms 1ms", "invalid --slot"},
		{"distribution file path", "distribution /etc/passwd 5ms 1ms", "invalid --slot"},
		{"smuggle delay", "5ms 10ms delay 1ms", "invalid --slot"},
		{"smuggle ecn", "5ms 10ms ecn", "invalid --slot"},
		{"smuggle rate", "5ms rate 10mbit", "invalid --slot"},
		{"shell metachar", "5ms;rm", "invalid --slot"},
		{"missing unit", "5", "invalid --slot"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NetemSlot(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
