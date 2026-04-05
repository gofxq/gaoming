package http

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"sort"
	"time"

	"github.com/gofxq/gaoming/pkg/state"
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

	_, updates, cancel := s.svc.SubscribeHosts()
	defer cancel()

	lastVersions := make(map[string]int64)
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case items, ok := <-updates:
			if !ok {
				return
			}

			now := time.Now().UTC()
			if len(lastVersions) == 0 {
				payload, err := json.Marshal(hostSyncPayload{
					Items:      items,
					Histories:  s.svc.GetAllHostMetricHistory(),
					ServerTime: now,
				})
				if err != nil {
					continue
				}

				if _, err := fmt.Fprintf(w, "event: sync\ndata: %s\n\n", payload); err != nil {
					return
				}
				lastVersions = versionsByHost(items)
				flusher.Flush()
				continue
			}

			currentVersions := versionsByHost(items)
			sorted := append([]state.HostSnapshot(nil), items...)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].HostUID < sorted[j].HostUID
			})

			for _, item := range sorted {
				if lastVersions[item.HostUID] == item.Version {
					continue
				}

				payload, err := json.Marshal(hostUpsertPayload{
					Item:       item,
					History:    s.svc.GetHostMetricHistory(item.HostUID),
					ServerTime: now,
				})
				if err != nil {
					continue
				}

				if _, err := fmt.Fprintf(w, "event: host_upsert\ndata: %s\n\n", payload); err != nil {
					return
				}
			}

			for hostUID := range lastVersions {
				if _, ok := currentVersions[hostUID]; ok {
					continue
				}

				payload, err := json.Marshal(hostDeletePayload{
					HostUID:    hostUID,
					ServerTime: now,
				})
				if err != nil {
					continue
				}

				if _, err := fmt.Fprintf(w, "event: host_delete\ndata: %s\n\n", payload); err != nil {
					return
				}
			}

			lastVersions = currentVersions
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func versionsByHost(items []state.HostSnapshot) map[string]int64 {
	result := make(map[string]int64, len(items))
	for _, item := range items {
		result[item.HostUID] = item.Version
	}
	return result
}
