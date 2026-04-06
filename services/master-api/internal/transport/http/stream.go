package http

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
)

type hostSyncPayload struct {
	Items      []state.HostSnapshot                             `json:"items"`
	Latest     map[string]map[state.MetricKey]state.MetricPoint `json:"latest"`
	ServerTime time.Time                                        `json:"server_time"`
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

func (s *Server) handleHostStream(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodGet {
		nethttp.Error(w, "method not allowed", nethttp.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(nethttp.Flusher)
	if !ok {
		nethttp.Error(w, "streaming not supported", nethttp.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	items, err := s.svc.ListHosts(r.Context())
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	updates, err := s.svc.SubscribeHostEvents(r.Context())
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	now := time.Now().UTC()
	payload, err := json.Marshal(hostSyncPayload{
		Items:      items,
		Latest:     latestMetricPointsByHost(items),
		ServerTime: now,
	})
	if err == nil {
		if _, err := fmt.Fprintf(w, "event: sync\ndata: %s\n\n", payload); err != nil {
			return
		}
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-updates:
			if !ok {
				return
			}

			now := time.Now().UTC()
			switch event.Type {
			case service.HostEventDelete:
				payload, err := json.Marshal(hostDeletePayload{
					HostUID:    event.HostUID,
					ServerTime: now,
				})
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "event: host_delete\ndata: %s\n\n", payload); err != nil {
					return
				}
			case service.HostEventUpsert:
				if event.Snapshot == nil {
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
				if _, err := fmt.Fprintf(w, "event: host_upsert\ndata: %s\n\n", payload); err != nil {
					return
				}
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
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
