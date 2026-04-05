package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/gofxq/gaoming/pkg/contracts"
)

func Discover(region string, env string, role string) contracts.HostIdentity {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	ips := listIPs()
	primaryIP := "127.0.0.1"
	if len(ips) > 0 {
		primaryIP = ips[0]
	}

	hostUID := stableHostUID(hostname, machineID(), listMACs())

	return contracts.HostIdentity{
		HostUID:   hostUID,
		Hostname:  hostname,
		PrimaryIP: primaryIP,
		IPs:       ips,
		OSType:    runtime.GOOS,
		Arch:      runtime.GOARCH,
		Region:    region,
		Env:       env,
		Role:      role,
		Labels: map[string]string{
			"runtime": runtime.Version(),
		},
	}
}

func listIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}

	var ips []string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}

		if ipv4 := ipNet.IP.To4(); ipv4 != nil {
			ips = append(ips, ipv4.String())
		}
	}
	return ips
}

func listMACs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	macs := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		macs = append(macs, strings.ToLower(iface.HardwareAddr.String()))
	}
	sort.Strings(macs)
	return macs
}

func machineID() string {
	paths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		id := strings.TrimSpace(string(body))
		if id != "" {
			return id
		}
	}
	return ""
}

func stableHostUID(hostname string, machineID string, macs []string) string {
	parts := make([]string, 0, 5+len(macs))
	if machineID != "" {
		parts = append(parts, "machine-id="+machineID)
	}
	for _, mac := range macs {
		parts = append(parts, "mac="+mac)
	}

	// Keep weaker fallbacks only for environments where machine-level IDs are unavailable.
	if len(parts) == 0 {
		if hostname != "" {
			parts = append(parts, "hostname="+hostname)
		}
		parts = append(parts, "os="+runtime.GOOS, "arch="+runtime.GOARCH)
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "host-" + hex.EncodeToString(sum[:8])
}
