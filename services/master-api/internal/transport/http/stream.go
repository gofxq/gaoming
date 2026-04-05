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
	Items      []state.HostSnapshot                               `json:"items"`
	Histories  map[string]map[state.MetricKey][]state.MetricPoint `json:"histories"`
	ServerTime time.Time                                          `json:"server_time"`
}

type hostUpsertPayload struct {
	Item       state.HostSnapshot                      `json:"item"`
	History    map[state.MetricKey][]state.MetricPoint `json:"history"`
	ServerTime time.Time                               `json:"server_time"`
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
	histories, err := s.svc.GetAllHostMetricHistory(r.Context(), hostUIDs(items))
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
		Histories:  histories,
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
				history, err := s.svc.GetHostMetricHistory(r.Context(), event.Snapshot.HostUID)
				if err != nil {
					continue
				}
				payload, err := json.Marshal(hostUpsertPayload{
					Item:       *event.Snapshot,
					History:    history,
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

func hostUIDs(items []state.HostSnapshot) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.HostUID)
	}
	return result
}
