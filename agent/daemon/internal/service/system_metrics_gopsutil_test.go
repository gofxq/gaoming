package service

import (
	"testing"
	"time"
)

func TestCalculateRateInitialSampleReturnsZero(t *testing.T) {
	now := time.Now().UTC()

	a, b, curA, curB := calculateRate(0, 0, 100, 200, time.Time{}, now)

	if a != 0 || b != 0 {
		t.Fatalf("expected zero rates for initial sample, got %d and %d", a, b)
	}
	if curA != 100 || curB != 200 {
		t.Fatalf("expected current counters to pass through, got %d and %d", curA, curB)
	}
}

func TestCalculateRateCounterResetReturnsZero(t *testing.T) {
	prevAt := time.Now().UTC()
	now := prevAt.Add(2 * time.Second)

	a, b, curA, curB := calculateRate(200, 300, 100, 150, prevAt, now)

	if a != 0 || b != 0 {
		t.Fatalf("expected zero rates after counter reset, got %d and %d", a, b)
	}
	if curA != 100 || curB != 150 {
		t.Fatalf("expected reset counters to pass through, got %d and %d", curA, curB)
	}
}

func TestCalculateRateNormalDelta(t *testing.T) {
	prevAt := time.Now().UTC()
	now := prevAt.Add(2 * time.Second)

	a, b, _, _ := calculateRate(100, 40, 300, 100, prevAt, now)

	if a != 100 || b != 30 {
		t.Fatalf("expected rates 100 and 30, got %d and %d", a, b)
	}
}
