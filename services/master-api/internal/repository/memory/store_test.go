package memory

import (
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/state"
)

func TestRegisterAndHeartbeat(t *testing.T) {
	store := NewStore()

	snapshot, config := store.RegisterAgent(contracts.RegisterAgentRequest{
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

	updated, _, err := store.Heartbeat(contracts.HeartbeatRequest{
		HostUID: snapshot.HostUID,
		Digest: contracts.AgentDigest{
			CPUUsagePct: 23,
		},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if updated.CPUUsagePct != 23 {
		t.Fatalf("expected cpu usage to be updated, got %v", updated.CPUUsagePct)
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

	snapshot, _ := store.RegisterAgent(contracts.RegisterAgentRequest{
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

func TestHeartbeatStoresLoadHistory(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()

	snapshot, _ := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			Hostname:  "node-3",
			PrimaryIP: "10.0.0.3",
		},
	}, now)

	_, _, err := store.Heartbeat(contracts.HeartbeatRequest{
		HostUID: snapshot.HostUID,
		Digest: contracts.AgentDigest{
			Load1: 2.5,
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
}

func TestReconcileOfflineMarksHostOffline(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()

	snapshot, _ := store.RegisterAgent(contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			Hostname:  "node-4",
			PrimaryIP: "10.0.0.4",
		},
	}, now)

	changed := store.ReconcileOffline(now.Add(heartbeatOfflineThreshold + time.Second))
	if changed != 1 {
		t.Fatalf("expected one offline reconciliation, got %d", changed)
	}

	updated, ok := store.GetHost(snapshot.HostUID)
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
