package http

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
)

type hostSyncPayload struct {
	Items      []state.HostSnapshot                               `json:"items"`
	Histories  map[string]map[state.MetricKey][]state.MetricPoint `json:"histories,omitempty"`
	Latest     map[string]map[state.MetricKey]state.MetricPoint   `json:"latest"`
	ServerTime time.Time                                          `json:"server_time"`
}

type hostUpsertPayload struct {
	Item       state.HostSnapshot                    `json:"item"`
	Latest     map[state.MetricKey]state.MetricPoint `json:"latest"`
	ServerTime time.Time                             `json:"server_time"`
}

type hostDeletePayload struct {
	HostUID    string    `json:"host_uid"`
	ServerTime time.Time `json:"server_time"`
}

func (s *Server) handleHostStream(c *gin.Context) {
	tenantCode := tenantCodeFromContext(c)

	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		c.JSON(500, gin.H{"error": "streaming not supported"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	items, err := s.svc.ListHosts(c.Request.Context(), tenantCode)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	histories, err := s.svc.GetAllHostMetricHistory(c.Request.Context(), hostUIDsFromSnapshots(items))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	updates, err := s.svc.SubscribeHostEvents(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	now := time.Now().UTC()
	payload, err := json.Marshal(hostSyncPayload{
		Items:      items,
		Histories:  histories,
		Latest:     latestMetricPointsByHost(items),
		ServerTime: now,
	})
	if err == nil {
		if _, err := fmt.Fprintf(c.Writer, "event: sync\ndata: %s\n\n", payload); err != nil {
			return
		}
		flusher.Flush()
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event, ok := <-updates:
			if !ok {
				return
			}

			now := time.Now().UTC()
			switch event.Type {
			case service.HostEventDelete:
				if tenantCode != "" {
					continue
				}
				payload, err := json.Marshal(hostDeletePayload{
					HostUID:    event.HostUID,
					ServerTime: now,
				})
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(c.Writer, "event: host_delete\ndata: %s\n\n", payload); err != nil {
					return
				}
			case service.HostEventUpsert:
				if event.Snapshot == nil {
					continue
				}
				if !matchesTenant(*event.Snapshot, tenantCode) {
					continue
				}
				payload, err := json.Marshal(hostUpsertPayload{
					Item:       *event.Snapshot,
					Latest:     latestMetricPointsFromSnapshot(*event.Snapshot),
					ServerTime: now,
				})
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(c.Writer, "event: host_upsert\ndata: %s\n\n", payload); err != nil {
					return
				}
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(c.Writer, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func hostUIDsFromSnapshots(items []state.HostSnapshot) []string {
	if len(items) == 0 {
		return nil
	}

	hostUIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.HostUID == "" {
			continue
		}
		hostUIDs = append(hostUIDs, item.HostUID)
	}
	return hostUIDs
}

func matchesTenant(snapshot state.HostSnapshot, tenantCode string) bool {
	if tenantCode == "" {
		return true
	}
	return snapshot.TenantCode == tenantCode
}

func latestMetricPointsByHost(items []state.HostSnapshot) map[string]map[state.MetricKey]state.MetricPoint {
	if len(items) == 0 {
		return nil
	}

	latest := make(map[string]map[state.MetricKey]state.MetricPoint, len(items))
	for _, item := range items {
		points := latestMetricPointsFromSnapshot(item)
		if len(points) == 0 {
			continue
		}
		latest[item.HostUID] = points
	}
	if len(latest) == 0 {
		return nil
	}
	return latest
}

func latestMetricPointsFromSnapshot(snapshot state.HostSnapshot) map[state.MetricKey]state.MetricPoint {
	if snapshot.LastMetricAt.IsZero() {
		return nil
	}

	ts := snapshot.LastMetricAt.UTC()
	return map[state.MetricKey]state.MetricPoint{
		state.MetricCPUUsagePct:       {TS: ts, Value: snapshot.CPUUsagePct},
		state.MetricMemUsedPct:        {TS: ts, Value: snapshot.MemUsedPct},
		state.MetricMemAvailableBytes: {TS: ts, Value: float64(snapshot.MemAvailableBytes)},
		state.MetricSwapUsedPct:       {TS: ts, Value: snapshot.SwapUsedPct},
		state.MetricDiskUsedPct:       {TS: ts, Value: snapshot.DiskUsedPct},
		state.MetricDiskFreeBytes:     {TS: ts, Value: float64(snapshot.DiskFreeBytes)},
		state.MetricDiskInodesUsedPct: {TS: ts, Value: snapshot.DiskInodesUsedPct},
		state.MetricDiskReadBPS:       {TS: ts, Value: float64(snapshot.DiskReadBPS)},
		state.MetricDiskWriteBPS:      {TS: ts, Value: float64(snapshot.DiskWriteBPS)},
		state.MetricDiskReadIOPS:      {TS: ts, Value: float64(snapshot.DiskReadIOPS)},
		state.MetricDiskWriteIOPS:     {TS: ts, Value: float64(snapshot.DiskWriteIOPS)},
		state.MetricLoad1:             {TS: ts, Value: snapshot.Load1},
		state.MetricNetRxBPS:          {TS: ts, Value: float64(snapshot.NetRxBPS)},
		state.MetricNetTxBPS:          {TS: ts, Value: float64(snapshot.NetTxBPS)},
		state.MetricNetRxPacketsPS:    {TS: ts, Value: float64(snapshot.NetRxPacketsPS)},
		state.MetricNetTxPacketsPS:    {TS: ts, Value: float64(snapshot.NetTxPacketsPS)},
	}
}
