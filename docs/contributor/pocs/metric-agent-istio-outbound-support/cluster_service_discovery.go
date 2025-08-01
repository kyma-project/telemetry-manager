package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func isClusterLocalServiceReachable(rawURL string, kubeDNS string) error {
	u, err := url.Parse(rawURL)

	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := u.Hostname()

	if host == "" {
		// try split host when a scheme is not present
		host, _, err = net.SplitHostPort(rawURL)
		if err != nil {
			return fmt.Errorf("invalid host in URL: %w", err)
		}
	}
	// Step 1: Check cluster DNS pattern
	if strings.HasSuffix(host, ".svc") || strings.HasSuffix(host, ".svc.cluster.local") {
		fmt.Println("DNS pattern suggests cluster-local service")
	}

	// Step 2: DNS lookup
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("DNS resolution failed: %w", err)
	}

	/*

		// Step 2: Custom DNS resolver (optional)
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return net.Dial("udp", kubeDNS)
			},
		}

		// Resolve using custom DNS
		ips, err := resolver.LookupHost(context.Background(), host)
		if err != nil {
			return fmt.Errorf("DNS resolution failed: %w", err)
		}
	*/

	nets := []net.IPNet{
		{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},     // RFC1918 (private address space)
		{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},  // RFC1918 (private address space)
		{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)}, // RFC1918 (private address space)
		{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)},  // RFC6598 (shared address space)
	}
	for _, ip := range ips {
		if isPrivateIPOrShared(net.ParseIP(ip), nets) {
			fmt.Printf("Resolved IP %s is private or shared (likely cluster-local)\n", ip)
		} else {
			fmt.Printf("Resolved IP %s is public\n", ip)
		}
	}

	return nil
}

// Check if an IP address is private (RFC1918 or RFC6598 ranges)
func isPrivateIPOrShared(ip net.IP, nets []net.IPNet) bool {
	for _, net := range nets {
		if net.Contains(ip) {
			return true
		}
	}
	return false
}

func main() {
	kubeDNS := "10.43.0.10:53" // cluster kube DNS service IP and port, optional

	urls := []string{
		"my-service.my-namespace.svc.cluster.local:4317", // Kubernetes FQDN service
		"http://10.43.2.3:8080",                          // RFC1918
		"https://google.com",                             // Public
		"http://100.64.0.38:4318",                        // RFC6598
		"http://8.8.8.8",                                 // Public
	}

	for _, rawURL := range urls {
		fmt.Printf("\nChecking URL: %s\n", rawURL)
		isClusterLocalServiceReachable(rawURL, kubeDNS)
	}
}
