package service

import "time"

type systemMetrics struct {
	CPUUsagePct  float64
	MemUsedPct   float64
	DiskUsedPct  float64
	DiskReadBPS  int64
	DiskWriteBPS int64
	Load1        float64
	NetRxBPS     int64
	NetTxBPS     int64
}

type systemSampler interface {
	Sample(now time.Time) systemMetrics
}
