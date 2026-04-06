package http

import (
	"errors"
	nethttp "net/http"
	"strings"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/httpx"
	"github.com/gofxq/gaoming/services/master-api/internal/repository"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
)

type Server struct {
	svc *service.Service
}

func NewServer(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) Handler() nethttp.Handler {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("/master/", s.handleDashboard)
	mux.HandleFunc("/master/healthz", s.handleHealth)
	mux.HandleFunc("/master/api/v1/stream/hosts", s.handleHostStream)
	mux.HandleFunc("/master/api/v1/agents/register", s.handleRegisterAgent)
	mux.HandleFunc("/master/api/v1/agents/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/master/api/v1/hosts", s.handleListHosts)
	mux.HandleFunc("/master/api/v1/hosts/", s.handleGetHost)
	mux.HandleFunc("/master/api/v1/ops/maintenance", s.handleCreateMaintenance)
	mux.HandleFunc("/master/api/v1/ops/alerts/", s.handleAckAlert)
	return mux
}

func (s *Server) handleHealth(w nethttp.ResponseWriter, _ *nethttp.Request) {
	httpx.WriteJSON(w, nethttp.StatusOK, s.svc.Health())
}

func (s *Server) handleRegisterAgent(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req contracts.RegisterAgentRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, nethttp.StatusBadRequest, err.Error())
		return
	}

	resp, err := s.svc.RegisterAgent(r.Context(), req)
	if err != nil {
		httpx.Error(w, nethttp.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, nethttp.StatusOK, resp)
}

func (s *Server) handleHeartbeat(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req contracts.HeartbeatRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, nethttp.StatusBadRequest, err.Error())
		return
	}

	resp, err := s.svc.Heartbeat(r.Context(), req)
	if err != nil {
		if errors.Is(err, repository.ErrHostNotFound) {
			httpx.Error(w, nethttp.StatusNotFound, err.Error())
			return
		}
		httpx.Error(w, nethttp.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, nethttp.StatusOK, resp)
}

func (s *Server) handleListHosts(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodGet {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.svc.ListHosts(r.Context())
	if err != nil {
		httpx.Error(w, nethttp.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, nethttp.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetHost(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodGet {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	hostUID := strings.TrimPrefix(r.URL.Path, "/master/api/v1/hosts/")
	if hostUID == "" {
		httpx.Error(w, nethttp.StatusBadRequest, "missing host uid")
		return
	}

	host, ok, err := s.svc.GetHost(r.Context(), hostUID)
	if err != nil {
		httpx.Error(w, nethttp.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		httpx.Error(w, nethttp.StatusNotFound, "host not found")
		return
	}

	httpx.WriteJSON(w, nethttp.StatusOK, host)
}

func (s *Server) handleCreateMaintenance(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req contracts.CreateMaintenanceWindowRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, nethttp.StatusBadRequest, err.Error())
		return
	}

	resp, err := s.svc.CreateMaintenance(r.Context(), req)
	if err != nil {
		httpx.Error(w, nethttp.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, nethttp.StatusCreated, resp)
}

func (s *Server) handleAckAlert(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		httpx.Error(w, nethttp.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/master/api/v1/ops/alerts/")
	alertID := strings.TrimSuffix(path, "/ack")
	if alertID == "" || !strings.HasSuffix(r.URL.Path, "/ack") {
		httpx.Error(w, nethttp.StatusBadRequest, "invalid alert ack path")
		return
	}

	var req contracts.AckAlertRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, nethttp.StatusBadRequest, err.Error())
		return
	}

	resp, err := s.svc.AckAlert(r.Context(), alertID, req)
	if err != nil {
		httpx.Error(w, nethttp.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, nethttp.StatusOK, resp)
}
