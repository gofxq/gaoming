package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
)

type gopsutilSystemSampler struct {
	prevNetRx       uint64
	prevNetTx       uint64
	prevDiskRead    uint64
	prevDiskWrite   uint64
	prevCollectedAt time.Time
	rootPath        string
}

func newGopsutilSystemSampler() systemSampler {
	return &gopsutilSystemSampler{
		rootPath: systemRootPath(),
	}
}

func (s *gopsutilSystemSampler) Sample(now time.Time) systemMetrics {
	rxBPS, txBPS, rxBytes, txBytes := readNetRate(s.prevNetRx, s.prevNetTx, s.prevCollectedAt, now)
	diskReadBPS, diskWriteBPS, diskReadBytes, diskWriteBytes := readDiskRate(s.prevDiskRead, s.prevDiskWrite, s.prevCollectedAt, now)

	s.prevNetRx = rxBytes
	s.prevNetTx = txBytes
	s.prevDiskRead = diskReadBytes
	s.prevDiskWrite = diskWriteBytes
	s.prevCollectedAt = now

	return systemMetrics{
		CPUUsagePct:  readCPUUsage(),
		MemUsedPct:   readMemUsage(),
		DiskUsedPct:  readDiskUsage(s.rootPath),
		DiskReadBPS:  diskReadBPS,
		DiskWriteBPS: diskWriteBPS,
		Load1:        readLoad1(),
		NetRxBPS:     rxBPS,
		NetTxBPS:     txBPS,
	}
}

func readCPUUsage() float64 {
	values, err := cpu.Percent(0, false)
	if err != nil || len(values) == 0 {
		return 0
	}
	return clampPercent(values[0])
}

func readMemUsage() float64 {
	stats, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return clampPercent(stats.UsedPercent)
}

func readDiskUsage(path string) float64 {
	stats, err := disk.Usage(path)
	if err != nil {
		return 0
	}
	return clampPercent(stats.UsedPercent)
}

func readLoad1() float64 {
	stats, err := load.Avg()
	if err != nil {
		return 0
	}
	if stats.Load1 < 0 {
		return 0
	}
	return stats.Load1
}

func readNetRate(prevRx uint64, prevTx uint64, prevAt time.Time, now time.Time) (int64, int64, uint64, uint64) {
	stats, err := gnet.IOCounters(false)
	if err != nil || len(stats) == 0 {
		return 0, 0, prevRx, prevTx
	}

	return calculateRate(prevRx, prevTx, stats[0].BytesRecv, stats[0].BytesSent, prevAt, now)
}

func readDiskRate(prevRead uint64, prevWrite uint64, prevAt time.Time, now time.Time) (int64, int64, uint64, uint64) {
	stats, err := disk.IOCounters()
	if err != nil || len(stats) == 0 {
		return 0, 0, prevRead, prevWrite
	}

	readBytes, writeBytes := sumDiskCounters(stats)
	return calculateRate(prevRead, prevWrite, readBytes, writeBytes, prevAt, now)
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

func sumDiskCounters(stats map[string]disk.IOCountersStat) (uint64, uint64) {
	type totals struct {
		readBytes  uint64
		writeBytes uint64
	}

	byDevice := make(map[string]totals, len(stats))

	for name, stat := range stats {
		normalized := normalizeDiskName(name)
		if skipDiskCounter(normalized) {
			continue
		}

		current := byDevice[normalized]
		if stat.ReadBytes > current.readBytes {
			current.readBytes = stat.ReadBytes
		}
		if stat.WriteBytes > current.writeBytes {
			current.writeBytes = stat.WriteBytes
		}
		byDevice[normalized] = current
	}

	var readBytes uint64
	var writeBytes uint64
	for _, total := range byDevice {
		readBytes += total.readBytes
		writeBytes += total.writeBytes
	}

	return readBytes, writeBytes
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

func systemRootPath() string {
	if runtime.GOOS == "windows" {
		if drive := strings.TrimSpace(os.Getenv("SystemDrive")); drive != "" {
			return drive + `\`
		}
		return `C:\`
	}
	return string(filepath.Separator)
}
