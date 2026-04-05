package memory

import (
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
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
