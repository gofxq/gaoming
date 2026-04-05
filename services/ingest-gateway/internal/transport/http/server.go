package http

import (
	nethttp "net/http"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/httpx"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
)

type Server struct {
	svc *service.Service
}

func NewServer(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) Handler() nethttp.Handler {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v1/metrics", s.handleMetrics)
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/probes", s.handleProbes)
	mux.HandleFunc("/debug/counters", s.handleCounters)
	return mux
}

func (s *Server) handleHealth(w nethttp.ResponseWriter, _ *nethttp.Request) {
	httpx.WriteJSON(w, nethttp.StatusOK, s.svc.Health())
}

func (s *Server) handleMetrics(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req contracts.PushMetricBatchRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, nethttp.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, nethttp.StatusAccepted, s.svc.PushMetricBatch(req))
}

func (s *Server) handleEvents(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req contracts.PushEventBatchRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, nethttp.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, nethttp.StatusAccepted, s.svc.PushEventBatch(req))
}

func (s *Server) handleProbes(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req contracts.ReportProbeResultsRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, nethttp.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, nethttp.StatusAccepted, s.svc.ReportProbeResults(req))
}

func (s *Server) handleCounters(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodGet {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}
	httpx.WriteJSON(w, nethttp.StatusOK, s.svc.Stats())
}
