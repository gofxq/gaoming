package service

import "time"

type systemMetrics struct {
	CPUUsagePct       float64
	MemUsedPct        float64
	MemAvailableBytes int64
	SwapUsedPct       float64
	DiskUsedPct       float64
	DiskFreeBytes     int64
	DiskInodesUsedPct float64
	DiskReadBPS       int64
	DiskWriteBPS      int64
	DiskReadIOPS      int64
	DiskWriteIOPS     int64
	Load1             float64
	NetRxBPS          int64
	NetTxBPS          int64
	NetRxPacketsPS    int64
	NetTxPacketsPS    int64
}

type systemSampler interface {
	Sample(now time.Time) systemMetrics
}
