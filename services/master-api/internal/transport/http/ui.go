package http

import (
	_ "embed"
	nethttp "net/http"
)

//go:embed ui_index.html
var dashboardHTML []byte

func (s *Server) handleDashboard(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodGet {
		nethttp.Error(w, "method not allowed", nethttp.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path != "/master/" && r.URL.Path != "/master/ui/agents" {
		nethttp.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(nethttp.StatusOK)
	_, _ = w.Write(dashboardHTML)
}
