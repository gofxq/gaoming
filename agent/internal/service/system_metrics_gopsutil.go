package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
)

const maxCollectionDuration = 95 * time.Millisecond

type gopsutilSystemSampler struct {
	prevNetRx         uint64
	prevNetTx         uint64
	prevNetPacketsIn  uint64
	prevNetPacketsOut uint64
	prevDiskRead      uint64
	prevDiskWrite     uint64
	prevDiskReadOps   uint64
	prevDiskWriteOps  uint64
	prevNetAt         time.Time
	prevDiskAt        time.Time
	rootPath          string
}

func newGopsutilSystemSampler() systemSampler {
	return &gopsutilSystemSampler{
		rootPath: systemRootPath(),
	}
}

func (s *gopsutilSystemSampler) Sample(now time.Time) systemMetrics {
	ctx, cancel := context.WithTimeout(context.Background(), maxCollectionDuration)
	defer cancel()
	extraPoints, collectors, capabilities, structured := collectExtendedMetrics(ctx)
	metrics := systemMetrics{
		ExtraPoints:  extraPoints,
		Collectors:   collectors,
		Capabilities: capabilities,
		Structured:   structured,
	}
	if structured == nil {
		return metrics
	}
	if structured.Cpu != nil {
		metrics.CPUUsagePct = clampPercent(structured.Cpu.UsagePct)
	}
	if structured.Memory != nil {
		metrics.MemUsedPct = clampPercent(structured.Memory.UsedPct)
		metrics.MemAvailableBytes = clampUint64ToInt64(structured.Memory.AvailableBytes)
	}
	if structured.Swap != nil {
		metrics.SwapUsedPct = clampPercent(structured.Swap.UsedPct)
	}
	if structured.Load != nil && structured.Load.Load1 > 0 {
		metrics.Load1 = structured.Load.Load1
	}
	for _, filesystem := range structured.Filesystems {
		if filepath.Clean(filesystem.Mountpoint) != filepath.Clean(s.rootPath) {
			continue
		}
		metrics.DiskUsedPct = clampPercent(filesystem.UsedPct)
		metrics.DiskFreeBytes = clampUint64ToInt64(filesystem.FreeBytes)
		metrics.DiskInodesUsedPct = clampPercent(filesystem.InodesUsedPct)
		break
	}

	if len(structured.NetworkInterfaces) > 0 {
		var rxBytes, txBytes, rxPackets, txPackets uint64
		for _, network := range structured.NetworkInterfaces {
			rxBytes += network.BytesReceivedTotal
			txBytes += network.BytesSentTotal
			rxPackets += network.PacketsReceivedTotal
			txPackets += network.PacketsSentTotal
		}
		metrics.NetRxBPS, metrics.NetTxBPS, s.prevNetRx, s.prevNetTx = calculateRate(
			s.prevNetRx, s.prevNetTx, rxBytes, txBytes, s.prevNetAt, now,
		)
		metrics.NetRxPacketsPS, metrics.NetTxPacketsPS, s.prevNetPacketsIn, s.prevNetPacketsOut = calculateRate(
			s.prevNetPacketsIn, s.prevNetPacketsOut, rxPackets, txPackets, s.prevNetAt, now,
		)
		s.prevNetAt = now
	}
	if len(structured.Disks) > 0 {
		readBytes, writeBytes, readOps, writeOps := sumStructuredDiskCounters(structured.Disks)
		metrics.DiskReadBPS, metrics.DiskWriteBPS, s.prevDiskRead, s.prevDiskWrite = calculateRate(
			s.prevDiskRead, s.prevDiskWrite, readBytes, writeBytes, s.prevDiskAt, now,
		)
		metrics.DiskReadIOPS, metrics.DiskWriteIOPS, s.prevDiskReadOps, s.prevDiskWriteOps = calculateRate(
			s.prevDiskReadOps, s.prevDiskWriteOps, readOps, writeOps, s.prevDiskAt, now,
		)
		s.prevDiskAt = now
	}

	return metrics
}

func calculateRate(prevA uint64, prevB uint64, curA uint64, curB uint64, prevAt time.Time, now time.Time) (int64, int64, uint64, uint64) {
	if prevAt.IsZero() || now.Before(prevAt) || curA < prevA || curB < prevB {
		return 0, 0, curA, curB
	}

	seconds := now.Sub(prevAt).Seconds()
	if seconds <= 0 {
		return 0, 0, curA, curB
	}

	rateA := int64(float64(curA-prevA) / seconds)
	rateB := int64(float64(curB-prevB) / seconds)
	return rateA, rateB, curA, curB
}

func sumStructuredDiskCounters(stats []*monitorv1.DiskIOMetrics) (uint64, uint64, uint64, uint64) {
	type totals struct {
		readBytes, writeBytes, readOps, writeOps uint64
	}
	byDevice := make(map[string]totals, len(stats))
	for _, stat := range stats {
		if stat == nil {
			continue
		}
		name := normalizeDiskName(stat.Device)
		if skipDiskCounter(name) {
			continue
		}
		current := byDevice[name]
		if stat.ReadBytesTotal > current.readBytes {
			current.readBytes = stat.ReadBytesTotal
		}
		if stat.WriteBytesTotal > current.writeBytes {
			current.writeBytes = stat.WriteBytesTotal
		}
		if stat.ReadsTotal > current.readOps {
			current.readOps = stat.ReadsTotal
		}
		if stat.WritesTotal > current.writeOps {
			current.writeOps = stat.WritesTotal
		}
		byDevice[name] = current
	}
	var readBytes, writeBytes, readOps, writeOps uint64
	for _, stat := range byDevice {
		readBytes += stat.readBytes
		writeBytes += stat.writeBytes
		readOps += stat.readOps
		writeOps += stat.writeOps
	}
	return readBytes, writeBytes, readOps, writeOps
}

func normalizeDiskName(name string) string {
	name = strings.ToLower(strings.TrimSpace(filepath.Base(name)))
	if runtime.GOOS != "linux" {
		return name
	}

	switch {
	case strings.HasPrefix(name, "nvme"), strings.HasPrefix(name, "mmcblk"):
		if idx := strings.LastIndex(name, "p"); idx > 0 && isAllDigits(name[idx+1:]) {
			return name[:idx]
		}
	case strings.HasPrefix(name, "sd"), strings.HasPrefix(name, "vd"), strings.HasPrefix(name, "xvd"), strings.HasPrefix(name, "hd"):
		for idx := len(name) - 1; idx >= 0; idx-- {
			if name[idx] < '0' || name[idx] > '9' {
				return name[:idx+1]
			}
		}
	}

	return name
}

func skipDiskCounter(name string) bool {
	switch {
	case name == "":
		return true
	case strings.HasPrefix(name, "loop"):
		return true
	case strings.HasPrefix(name, "ram"):
		return true
	case strings.HasPrefix(name, "fd"):
		return true
	default:
		return false
	}
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}

	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func clampUint64ToInt64(value uint64) int64 {
	if value > uint64(^uint64(0)>>1) {
		return int64(^uint64(0) >> 1)
	}
	return int64(value)
}

func systemRootPath() string {
	if runtime.GOOS == "windows" {
		if drive := strings.TrimSpace(os.Getenv("SystemDrive")); drive != "" {
			return drive + `\`
		}
		return `C:\`
	}
	return string(filepath.Separator)
}
