package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/internal/testutil"
	"github.com/gofxq/gaoming/server/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestUnifiedBackendNativeIntegration(t *testing.T) {
	fixture := testutil.Postgres(t)
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("set TEST_REDIS_ADDR for native unified integration")
	}
	var schema string
	if err := fixture.QueryRow("SELECT current_schema()").Scan(&schema); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(os.Getenv("TEST_POSTGRES_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	path := filepath.Join(t.TempDir(), "server.yml")
	body := fmt.Sprintf("postgres_dsn: %q\nredis_addr: %q\nredis_password: %q\nredis_db: 15\nhttp_addr: '0.0.0.0:0'\ngrpc_addr: '0.0.0.0:0'\nsession_cookie_secure: false\nsetup_token: %q\nprobe_interval_sec: 1\nredis_namespace: %q\n", u.String(), redisAddr, os.Getenv("TEST_REDIS_PASSWORD"), strings.Repeat("s", 32), "gaoming:"+schema)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAOMING_LOG_DIR", t.TempDir())
	t.Setenv("GAOMING_ADMIN_PASSWORD", "integration-test-password")
	a, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var cursor uint64
		for {
			keys, next, err := a.redisClient.Scan(ctx, cursor, "gaoming:"+schema+":*", 100).Result()
			if err != nil {
				t.Error("clean isolated Redis namespace", err)
				return
			}
			if len(keys) > 0 {
				if err := a.redisClient.Del(ctx, keys...).Err(); err != nil {
					t.Error(err)
				}
			}
			cursor = next
			if cursor == 0 {
				return
			}
		}
	})
	observer := a.redisClient.Subscribe(context.Background(), "gaoming:master-api:host-events")
	if _, err := observer.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { observer.Close() })

	hl, err := net.Listen("tcp", cfg.Master.HTTPAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hl.Close() })
	gl, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { gl.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.serve(ctx, hl, gl) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(7 * time.Second):
			t.Error("backend did not stop")
		}
	})
	base := localHTTPBase(hl.Addr().String())
	client := &http.Client{Timeout: 3 * time.Second}
	for _, path := range []string{"/master/healthz", "/ingest/healthz"} {
		resp, err := client.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("health %s: %d", path, resp.StatusCode)
		}
	}
	resp, err := client.Get(base + "/master/api/v1/hosts")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("anonymous private read accepted: %d", resp.StatusCode)
	}
	payload := map[string]string{"tenant_code": "default", "username": "admin", "password": "integration-test-password"}
	raw, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", base+"/master/api/v1/auth/login", bytes.NewReader(raw))
	req.Header.Set("Origin", base)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	cookies := resp.Cookies()
	resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		t.Fatalf("startup administrator login failed: %d", resp.StatusCode)
	}
	conn, err := grpc.NewClient(gl.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer callCancel()
	host := &monitorv1.HostIdentity{HostUid: "unified-" + schema, Hostname: "native-test", PrimaryIp: "127.0.0.1", TenantCode: "default"}
	agent := &monitorv1.AgentMetadata{AgentId: "agent-" + schema, Version: "integration"}
	registered, err := monitorv1.NewAgentControlServiceClient(conn).RegisterAgent(callCtx, &monitorv1.RegisterAgentRequest{Host: host, Agent: agent})
	if err != nil {
		t.Fatal(err)
	}
	host.HostUid = registered.HostUid
	stream, err := monitorv1.NewMetricsIngestServiceClient(conn).StreamMetricBatches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&monitorv1.PushMetricBatchRequest{HostUid: host.HostUid, AgentId: agent.AgentId, Host: host, Agent: agent, BatchSeq: 1, CollectedAt: timestamppb.Now(), Points: []*monitorv1.MetricPoint{{Name: "host_cpu_usage_pct", Value: 42, Ts: timestamppb.Now()}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-observer.Channel():
		t.Fatal("integration published a host event into the default application channel")
	case <-time.After(150 * time.Millisecond):
	}

	req, _ = http.NewRequest("GET", base+"/master/api/v1/hosts", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var hosts struct {
		Items []struct {
			HostUID string  `json:"host_uid"`
			CPU     float64 `json:"cpu_usage_pct"`
		} `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&hosts)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || len(hosts.Items) != 1 || hosts.Items[0].HostUID != host.HostUid || hosts.Items[0].CPU != 42 {
		t.Fatal("HTTP master did not observe gRPC ingestion")
	}
	resp, err = client.Get(base + "/ingest/debug/counters")
	if err != nil {
		t.Fatal(err)
	}
	var counters struct {
		Metrics int `json:"metric_batches"`
		Probes  int `json:"probe_reports"`
	}
	err = json.NewDecoder(resp.Body).Decode(&counters)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if counters.Metrics != 1 {
		t.Fatal("metric counter not updated")
	}
	deadline := time.Now().Add(3 * time.Second)
	for counters.Probes == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		resp, err = client.Get(base + "/ingest/debug/counters")
		if err != nil {
			t.Fatal(err)
		}
		err = json.NewDecoder(resp.Body).Decode(&counters)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if counters.Probes == 0 {
		t.Fatal("integrated probe worker did not report")
	}
	// Keep both protocols' streams open while cancellation forces bounded shutdown.
	req, _ = http.NewRequest("GET", base+"/master/api/v1/stream/hosts", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	sse, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer sse.Body.Close()
	if sse.StatusCode != 200 {
		t.Fatalf("SSE failed: %d", sse.StatusCode)
	}
	streamClosed := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, sse.Body); close(streamClosed) }()
	started := time.Now()
	cancel()
	select {
	case err := <-done:
		done <- err
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("streams prevented bounded shutdown")
	}
	select {
	case <-streamClosed:
	case <-time.After(time.Second):
		t.Fatal("SSE remained open after backend shutdown")
	}
	if time.Since(started) > 6*time.Second {
		t.Fatal("shutdown exceeded deadline")
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("gRPC stream remained active")
	}
}
