package http

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/server/internal/master/service"
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
	principal := principalFromContext(c)

	items, err := s.svc.ReadHosts(c.Request.Context(), principal)
	if err != nil {
		c.JSON(500, gin.H{"error": "host state unavailable"})
		return
	}
	var histories map[string]map[state.MetricKey][]state.MetricPoint
	if principal.AllowsField("metrics") && len(items) > 0 {
		histories, err = s.svc.GetAllHostMetricHistory(c.Request.Context(), hostUIDsFromSnapshots(items))
		if err != nil {
			c.JSON(500, gin.H{"error": "history unavailable"})
			return
		}
		filtered := make(map[string]map[state.MetricKey][]state.MetricPoint, len(items))
		for _, item := range items {
			if history, ok := histories[item.HostUID]; ok {
				points := make(map[state.MetricKey][]state.MetricPoint)
				for _, key := range repository.MetricKeys() {
					if values, ok := history[key]; ok {
						points[key] = values
					}
				}
				filtered[item.HostUID] = points
			}
		}
		histories = filtered
	}
	updates, err := s.svc.SubscribeHostEvents(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": "live updates unavailable"})
		return
	}

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	recheck := time.NewTicker(s.svc.ReadAccessRecheckInterval())
	defer recheck.Stop()
	if !s.recheckReadAccess(c, principal) {
		writeError(c, service.ErrUnauthorized)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	now := time.Now().UTC()
	var latest map[string]map[state.MetricKey]state.MetricPoint
	if principal.AllowsField("metrics") {
		latest = latestMetricPointsByHost(items)
	}
	payload, err := json.Marshal(gin.H{"items": service.ProjectHosts(principal, items), "histories": histories, "latest": latest, "server_time": now})
	if err == nil {
		if err := writeStreamEvent(c, "sync", payload); err != nil {
			return
		}
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-recheck.C:
			if !s.recheckReadAccess(c, principal) {
				return
			}
		case event, ok := <-updates:
			if !ok {
				return
			}

			now := time.Now().UTC()
			if !s.recheckReadAccess(c, principal) {
				return
			}
			switch event.Type {
			case service.HostEventDelete:
				// Existing deletion messages have no trusted tenant. Fail closed
				// until T09 introduces tenant-bearing, versioned deletion events.
				continue
			case service.HostEventUpsert:
				if event.Snapshot == nil {
					continue
				}
				if !principal.AllowsHost(event.Snapshot.TenantCode, event.Snapshot.HostUID) {
					continue
				}
				var latest map[state.MetricKey]state.MetricPoint
				if principal.AllowsField("metrics") {
					latest = latestMetricPointsFromSnapshot(*event.Snapshot)
				}
				payload, err := json.Marshal(gin.H{"item": service.ProjectHost(principal, *event.Snapshot), "latest": latest, "server_time": now})
				if err != nil {
					continue
				}
				if err := writeStreamEvent(c, "host_upsert", payload); err != nil {
					return
				}
			}
		case <-heartbeat.C:
			controller := nethttp.NewResponseController(c.Writer)
			_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := fmt.Fprint(c.Writer, ": keep-alive\n\n"); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}

func writeStreamEvent(c *gin.Context, event string, payload []byte) error {
	controller := nethttp.NewResponseController(c.Writer)
	_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return err
	}
	return controller.Flush()
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
