package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofxq/gaoming/pkg/contracts"
)

const (
	maxMetricPoints           = 4096
	maxMetricNameBytes        = 128
	maxLabelsPerPoint         = 16
	maxLabelKeyBytes          = 64
	maxLabelValueBytes        = 256
	maxCollectors             = 64
	maxCollectorMetricNames   = 4096
	maxMetricBatchStringBytes = 1 << 20
)

var ErrInvalidMetricBatch = errors.New("invalid metric batch")

func validateMetricBatch(req contracts.PushMetricBatchRequest) error {
	if len(req.Points) > maxMetricPoints {
		return invalidMetricBatch("point count %d exceeds %d", len(req.Points), maxMetricPoints)
	}
	if len(req.Collectors) > maxCollectors {
		return invalidMetricBatch("collector count %d exceeds %d", len(req.Collectors), maxCollectors)
	}

	stringBytes := 0
	for index, point := range req.Points {
		if !validIdentifier(point.Name, maxMetricNameBytes) {
			return invalidMetricBatch("point %d has an invalid name", index)
		}
		if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) {
			return invalidMetricBatch("point %d has a non-finite value", index)
		}
		if len(point.Labels) > maxLabelsPerPoint {
			return invalidMetricBatch("point %d label count %d exceeds %d", index, len(point.Labels), maxLabelsPerPoint)
		}
		stringBytes += len(point.Name)
		for key, value := range point.Labels {
			if !validIdentifier(key, maxLabelKeyBytes) {
				return invalidMetricBatch("point %d has an invalid label key", index)
			}
			if !validText(value, maxLabelValueBytes) {
				return invalidMetricBatch("point %d label %q has an invalid value", index, key)
			}
			stringBytes += len(key) + len(value)
		}
	}

	collectorMetricNames := 0
	for index, collector := range req.Collectors {
		if !validIdentifier(collector.Name, maxMetricNameBytes) {
			return invalidMetricBatch("collector %d has an invalid name", index)
		}
		if collector.State < contracts.CollectorStateUnspecified || collector.State > contracts.CollectorStateDegraded {
			return invalidMetricBatch("collector %q has an invalid state", collector.Name)
		}
		if collector.DurationMicros < 0 {
			return invalidMetricBatch("collector %q has a negative duration", collector.Name)
		}
		stringBytes += len(collector.Name)
		collectorMetricNames += len(collector.MetricNames)
		if collectorMetricNames > maxCollectorMetricNames {
			return invalidMetricBatch("collector metric name count exceeds %d", maxCollectorMetricNames)
		}
		for _, name := range collector.MetricNames {
			if !validIdentifier(name, maxMetricNameBytes) {
				return invalidMetricBatch("collector %q has an invalid metric name", collector.Name)
			}
			stringBytes += len(name)
		}
	}

	if stringBytes > maxMetricBatchStringBytes {
		return invalidMetricBatch("string data size %d exceeds %d bytes", stringBytes, maxMetricBatchStringBytes)
	}
	return nil
}

func validIdentifier(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || strings.ContainsRune("_:-.", char) {
			continue
		}
		return false
	}
	return true
}

func validText(value string, maxBytes int) bool {
	if len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func invalidMetricBatch(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidMetricBatch, fmt.Sprintf(format, args...))
}
