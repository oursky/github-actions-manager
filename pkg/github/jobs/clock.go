package jobs

import "time"

type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time                         { return time.Now() }
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

type TestClock struct{}

func (TestClock) Now() time.Time                         { return time.Now() }
func (TestClock) After(d time.Duration) <-chan time.Time { return time.After(0) }
