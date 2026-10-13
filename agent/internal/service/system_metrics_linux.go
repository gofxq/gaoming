//go:build linux

package service

func newSystemSampler() systemSampler {
	return newGopsutilSystemSampler()
}
