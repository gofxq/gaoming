package identity

import (
	"net"
	"os"
	"runtime"

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

	return contracts.HostIdentity{
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
