// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"time"

	"gioui.org/gpu/backend"
)

type timers struct {
	backend backend.Device
	timers  []*timer
}

type timer struct {
	Elapsed time.Duration
	backend backend.Device
	timer   backend.Timer
	state   timerState
}

type timerState uint8

const (
	timerIdle timerState = iota
	timerRunning
	timerWaiting
)

func newTimers(b backend.Device) *timers {
	return &timers{
		backend: b,
	}
}

func (t *timers) newTimer() *timer {
	if t == nil {
		return nil
	}
	tt := &timer{
		backend: t.backend,
		timer:   t.backend.NewTimer(),
	}
	t.timers = append(t.timers, tt)
	return tt
}

func (t *timer) begin() {
	if t == nil || t.state != timerIdle {
		return
	}
	t.timer.Begin()
	t.state = timerRunning
}

func (t *timer) end() {
	if t == nil || t.state != timerRunning {
		return
	}
	t.timer.End()
	t.state = timerWaiting
}

func (t *timers) ready() bool {
	if t == nil {
		return false
	}
	for _, tt := range t.timers {
		switch tt.state {
		case timerIdle:
			continue
		case timerRunning:
			return false
		}
		d, ok := tt.timer.Duration()
		if !ok {
			return false
		}
		tt.state = timerIdle
		tt.Elapsed = d
	}
	return t.backend.IsTimeContinuous()
}

func (t *timers) release() {
	if t == nil {
		return
	}
	for _, tt := range t.timers {
		tt.timer.Release()
	}
	t.timers = nil
}
