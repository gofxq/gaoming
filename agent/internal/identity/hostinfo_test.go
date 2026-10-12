package identity

import (
	"testing"

	"github.com/gofxq/gaoming/pkg/contracts"
)

func TestStableHostUIDIsDeterministic(t *testing.T) {
	got1 := stableHostUID(
		"node-a",
		"machine-123",
		[]string{"00:11:22:33:44:55", "66:77:88:99:aa:bb"},
	)
	got2 := stableHostUID(
		"node-a",
		"machine-123",
		[]string{"00:11:22:33:44:55", "66:77:88:99:aa:bb"},
	)

	if got1 != got2 {
		t.Fatalf("expected deterministic host uid, got %q and %q", got1, got2)
	}
	if got1 == "" {
		t.Fatal("expected non-empty host uid")
	}
}

func TestStableHostUIDChangesWithFingerprint(t *testing.T) {
	got1 := stableHostUID(
		"node-a",
		"machine-123",
		[]string{"00:11:22:33:44:55"},
	)
	got2 := stableHostUID(
		"node-a",
		"machine-456",
		[]string{"00:11:22:33:44:55"},
	)

	if got1 == got2 {
		t.Fatalf("expected different host uid for different machine fingerprint, got %q", got1)
	}
}

func TestStableHostUIDFallsBackWithoutMachineIDOrMAC(t *testing.T) {
	got := stableHostUID("node-a", "", nil)
	if got == "" {
		t.Fatal("expected fallback host uid")
	}
}

func TestWithHostnameOverridesHostnameAndUID(t *testing.T) {
	base := contracts.HostIdentity{
		HostUID:  "host-original",
		Hostname: "original",
	}

	got := WithHostname(base, "air-agent-1")

	if got.Hostname != "air-agent-1" {
		t.Fatalf("unexpected hostname: %q", got.Hostname)
	}
	if got.HostUID == "" || got.HostUID == base.HostUID {
		t.Fatalf("expected derived host uid, got %q", got.HostUID)
	}
	if again := WithHostname(base, "air-agent-1"); again.HostUID != got.HostUID {
		t.Fatalf("expected stable host uid, got %q and %q", got.HostUID, again.HostUID)
	}
}
