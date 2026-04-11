package memory

import (
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/state"
)

func TestRegisterAndHeartbeat(t *testing.T) {
	store := NewStore()

	snapshot, config, tenantCode := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			Hostname:  "node-1",
			PrimaryIP: "10.0.0.1",
		},
	}, time.Now().UTC())

	if snapshot.HostUID == "" {
		t.Fatal("expected host uid to be assigned")
	}
	if config.HeartbeatIntervalSec == 0 {
		t.Fatal("expected default heartbeat interval")
	}
	if tenantCode == "" {
		t.Fatal("expected tenant code to be assigned")
	}

	updated, _, err := store.Heartbeat(contracts.HeartbeatRequest{
		HostUID: snapshot.HostUID,
		Digest: contracts.AgentDigest{
			CPUUsagePct:       23,
			MemAvailableBytes: 1024,
			DiskReadIOPS:      15,
			NetTxPacketsPS:    99,
		},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if updated.CPUUsagePct != 23 {
		t.Fatalf("expected cpu usage to be updated, got %v", updated.CPUUsagePct)
	}
	if updated.MemAvailableBytes != 1024 {
		t.Fatalf("expected mem available bytes to be updated, got %v", updated.MemAvailableBytes)
	}
	if updated.DiskReadIOPS != 15 {
		t.Fatalf("expected disk read iops to be updated, got %v", updated.DiskReadIOPS)
	}
	if updated.NetTxPacketsPS != 99 {
		t.Fatalf("expected net tx packets to be updated, got %v", updated.NetTxPacketsPS)
	}
}

func TestSubscribeGetsSnapshots(t *testing.T) {
	store := NewStore()

	_, ch, cancel := store.Subscribe()
	defer cancel()

	initial := <-ch
	if len(initial) != 0 {
		t.Fatalf("expected empty initial snapshot, got %d items", len(initial))
	}

	snapshot, _, _ := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			Hostname:  "node-2",
			PrimaryIP: "10.0.0.2",
		},
	}, time.Now().UTC())

	select {
	case update := <-ch:
		if len(update) != 1 {
			t.Fatalf("expected one item in update, got %d", len(update))
		}
		if update[0].HostUID != snapshot.HostUID {
			t.Fatalf("expected host uid %s, got %s", snapshot.HostUID, update[0].HostUID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for store update")
	}
}

func TestHeartbeatStoresMetricHistory(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()

	snapshot, _, _ := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			Hostname:  "node-3",
			PrimaryIP: "10.0.0.3",
		},
	}, now)

	_, _, err := store.Heartbeat(contracts.HeartbeatRequest{
		HostUID: snapshot.HostUID,
		Digest: contracts.AgentDigest{
			Load1:          2.5,
			DiskReadBPS:    1024,
			DiskWriteBPS:   2048,
			DiskReadIOPS:   12,
			NetRxPacketsPS: 33,
		},
	}, now.Add(5*time.Second))
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	history := store.GetMetricHistory(snapshot.HostUID)
	loadPoints := history[state.MetricLoad1]
	if len(loadPoints) != 1 {
		t.Fatalf("expected one load sample, got %d", len(loadPoints))
	}
	if loadPoints[0].Value != 2.5 {
		t.Fatalf("expected load value 2.5, got %v", loadPoints[0].Value)
	}

	readPoints := history[state.MetricDiskReadBPS]
	if len(readPoints) != 1 {
		t.Fatalf("expected one disk read sample, got %d", len(readPoints))
	}
	if readPoints[0].Value != 1024 {
		t.Fatalf("expected disk read value 1024, got %v", readPoints[0].Value)
	}

	readIOPSPoints := history[state.MetricDiskReadIOPS]
	if len(readIOPSPoints) != 1 {
		t.Fatalf("expected one disk read iops sample, got %d", len(readIOPSPoints))
	}
	if readIOPSPoints[0].Value != 12 {
		t.Fatalf("expected disk read iops value 12, got %v", readIOPSPoints[0].Value)
	}

	rxPacketPoints := history[state.MetricNetRxPacketsPS]
	if len(rxPacketPoints) != 1 {
		t.Fatalf("expected one net rx packets sample, got %d", len(rxPacketPoints))
	}
	if rxPacketPoints[0].Value != 33 {
		t.Fatalf("expected net rx packets value 33, got %v", rxPacketPoints[0].Value)
	}
}

func TestReconcileOfflineMarksHostOffline(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()

	snapshot, _, _ := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			Hostname:  "node-4",
			PrimaryIP: "10.0.0.4",
		},
	}, now)

	changed := store.ReconcileOffline(now.Add(heartbeatOfflineThreshold + time.Second))
	if changed != 1 {
		t.Fatalf("expected one offline reconciliation, got %d", changed)
	}

	updated, ok := store.GetHost(snapshot.HostUID, "")
	if !ok {
		t.Fatal("expected host to exist")
	}
	if updated.AgentState != state.Offline {
		t.Fatalf("expected agent state offline, got %v", updated.AgentState)
	}
	if updated.OverallState != state.Offline {
		t.Fatalf("expected overall state offline, got %v", updated.OverallState)
	}
}

func TestListHostsFiltersByTenant(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()

	first, _, firstTenant := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			TenantCode: "tenant-a",
			Hostname:   "node-a",
			PrimaryIP:  "10.0.0.11",
		},
	}, now)
	second, _, secondTenant := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			TenantCode: "tenant-b",
			Hostname:   "node-b",
			PrimaryIP:  "10.0.0.12",
		},
	}, now)

	items := store.ListHosts("tenant-a")
	if len(items) != 1 {
		t.Fatalf("expected one tenant-a host, got %d", len(items))
	}
	if items[0].HostUID != first.HostUID {
		t.Fatalf("expected host uid %s, got %s", first.HostUID, items[0].HostUID)
	}
	if items[0].TenantCode != firstTenant {
		t.Fatalf("expected tenant code %s, got %s", firstTenant, items[0].TenantCode)
	}

	if _, ok := store.GetHost(second.HostUID, firstTenant); ok {
		t.Fatal("expected cross-tenant host lookup to be rejected")
	}

	if got, ok := store.GetHost(second.HostUID, secondTenant); !ok || got.TenantCode != secondTenant {
		t.Fatalf("expected tenant-b host lookup to succeed, got %+v ok=%v", got, ok)
	}
}

func TestRegisterAgentRejectsCustomTenantWhenDisabled(t *testing.T) {
	store := NewStore(Config{AllowCustomTenantCode: false})

	_, _, tenantCode := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			TenantCode: "tenant-custom",
			Hostname:   "node-disabled",
			PrimaryIP:  "10.0.0.20",
		},
	}, time.Now().UTC())

	if tenantCode == "tenant-custom" {
		t.Fatalf("expected server-generated tenant code, got %q", tenantCode)
	}
	if tenantCode == "" {
		t.Fatal("expected tenant code to be assigned")
	}
}
