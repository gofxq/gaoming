//go:build !linux

package service

import "time"

type noopSystemSampler struct{}

func newSystemSampler() systemSampler {
	return noopSystemSampler{}
}

func (noopSystemSampler) Sample(time.Time) systemMetrics {
	return systemMetrics{}
}
