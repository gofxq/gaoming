package service

import (
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/state"
)

func digestFromSnapshot(h state.HostSnapshot) contracts.AgentDigest {
	return contracts.AgentDigest{CPUUsagePct: h.CPUUsagePct, MemUsedPct: h.MemUsedPct, MemAvailableBytes: h.MemAvailableBytes, SwapUsedPct: h.SwapUsedPct, DiskUsedPct: h.DiskUsedPct, DiskFreeBytes: h.DiskFreeBytes, DiskInodesUsedPct: h.DiskInodesUsedPct, DiskReadBPS: h.DiskReadBPS, DiskWriteBPS: h.DiskWriteBPS, DiskReadIOPS: h.DiskReadIOPS, DiskWriteIOPS: h.DiskWriteIOPS, Load1: h.Load1, NetRxBPS: h.NetRxBPS, NetTxBPS: h.NetTxBPS, NetRxPacketsPS: h.NetRxPacketsPS, NetTxPacketsPS: h.NetTxPacketsPS}
}
