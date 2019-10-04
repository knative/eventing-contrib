package common

import "time"

type Step struct {
	Rps uint
	Duration time.Duration
}

func (s Step) WaitTime() time.Duration {
	return time.Duration(float64(time.Second) / float64(s.Rps))
}