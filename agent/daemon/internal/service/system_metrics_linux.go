//go:build linux

package service

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type linuxSystemSampler struct {
	prevCPUTotal uint64
	prevCPUIdle  uint64
	prevNetRx    uint64
	prevNetTx    uint64
	prevNetAt    time.Time
}

func newSystemSampler() systemSampler {
	return &linuxSystemSampler{}
}

func (s *linuxSystemSampler) Sample(now time.Time) systemMetrics {
	cpuUsage, total, idle := readCPUUsage(s.prevCPUTotal, s.prevCPUIdle)
	s.prevCPUTotal = total
	s.prevCPUIdle = idle

	rxBPS, txBPS, rxBytes, txBytes := readNetRate(s.prevNetRx, s.prevNetTx, s.prevNetAt, now)
	s.prevNetRx = rxBytes
	s.prevNetTx = txBytes
	s.prevNetAt = now

	return systemMetrics{
		CPUUsagePct: cpuUsage,
		MemUsedPct:  readMemUsage(),
		DiskUsedPct: readDiskUsage("/"),
		Load1:       readLoad1(),
		NetRxBPS:    rxBPS,
		NetTxBPS:    txBPS,
	}
}

func readCPUUsage(prevTotal uint64, prevIdle uint64) (float64, uint64, uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, prevTotal, prevIdle
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() {
		return 0, prevTotal, prevIdle
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, prevTotal, prevIdle
	}

	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, prevTotal, prevIdle
		}
		values = append(values, value)
	}

	var total uint64
	for _, value := range values {
		total += value
	}

	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}

	if prevTotal == 0 || total <= prevTotal || idle < prevIdle {
		return 0, total, idle
	}

	deltaTotal := total - prevTotal
	deltaIdle := idle - prevIdle
	if deltaTotal == 0 {
		return 0, total, idle
	}

	usage := 100 * (1 - float64(deltaIdle)/float64(deltaTotal))
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, total, idle
}

func readMemUsage() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	var totalKB float64
	var availableKB float64
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}

		switch fields[0] {
		case "MemTotal:":
			totalKB = value
		case "MemAvailable:":
			availableKB = value
		}
	}

	if totalKB == 0 {
		return 0
	}

	usedPct := ((totalKB - availableKB) / totalKB) * 100
	if usedPct < 0 {
		return 0
	}
	return usedPct
}

func readLoad1() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}

	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return value
}

func readNetRate(prevRx uint64, prevTx uint64, prevAt time.Time, now time.Time) (int64, int64, uint64, uint64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0, prevRx, prevTx
	}

	var rxBytes uint64
	var txBytes uint64

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		rx, err1 := strconv.ParseUint(fields[0], 10, 64)
		tx, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}

		rxBytes += rx
		txBytes += tx
	}

	if prevAt.IsZero() || now.Before(prevAt) || rxBytes < prevRx || txBytes < prevTx {
		return 0, 0, rxBytes, txBytes
	}

	seconds := now.Sub(prevAt).Seconds()
	if seconds <= 0 {
		return 0, 0, rxBytes, txBytes
	}

	rxBPS := int64(float64(rxBytes-prevRx) / seconds)
	txBPS := int64(float64(txBytes-prevTx) / seconds)
	return rxBPS, txBPS, rxBytes, txBytes
}

func readDiskUsage(path string) float64 {
	if path == "" {
		path = string(filepath.Separator)
	}

	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return 0
	}

	if fs.Blocks == 0 {
		return 0
	}

	used := fs.Blocks - fs.Bavail
	return (float64(used) / float64(fs.Blocks)) * 100
}
