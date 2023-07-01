package ip

import (
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	IPDirectory = "/tmp/reserved_ips"
)

type AllocatedIP struct {
	Version string `json:"version"`
	Address string `json:"address"`
	Gateway string `json:"gateway"`
}

// Refer : https://github.com/morvencao/minicni

// AllocateIPs selects an available IP and a gateway IP from a CIDR.
func AllocateIPs(podCidr string) (string, string, error) {
	allIpAddr, err := GetAllIPs(podCidr)
	if err != nil {
		return "", "", fmt.Errorf("error getting all IPs: %w", err)
	}

	gwIpAddr := allIpAddr[0]

	reservedIPs, err := readIPsFromFile()
	if err != nil {
		return "", "", err
	}

	podIP, err := findAvailableIP(allIpAddr[1:], reservedIPs)
	if err != nil {
		return "", "", err
	}

	if err = writeIPsToFile(append(reservedIPs, podIP)); err != nil {
		return "", "", err
	}

	return podIP, gwIpAddr, nil
}

// readIPsFromFile reads the reserved IPs from file.
func readIPsFromFile() ([]string, error) {
	content, err := os.ReadFile(IPDirectory)
	if err != nil {
		return nil, fmt.Errorf("error reading reserved IPs file: %w", err)
	}

	return strings.Split(strings.TrimSpace(string(content)), "\n"), nil
}

// findAvailableIP finds an available IP from the given IPs that is not in the reserved IPs.
func findAvailableIP(ips, reservedIPs []string) (string, error) {
	for _, ip := range ips {
		if !isReserved(ip, reservedIPs) {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no available IPs")
}

// isReserved checks if the given IP is in the reserved IPs.
func isReserved(ip string, reservedIPs []string) bool {
	for _, rip := range reservedIPs {
		if ip == rip {
			return true
		}
	}

	return false
}

// writeIPsToFile writes the given IPs to file.
func writeIPsToFile(ips []string) error {
	if err := os.WriteFile(IPDirectory, []byte(strings.Join(ips, "\n")), 0600); err != nil {
		return fmt.Errorf("error writing to reserved IPs file: %w", err)
	}

	return nil
}

// ReturnIP removes a reserved IP from the reserved IPs file.
func ReturnIP(ip string) error {
	reservedIPs, err := readIPsFromFile()
	if err != nil {
		return err
	}

	if !removeIP(ip, reservedIPs) {
		return fmt.Errorf("IP %s is not reserved", ip)
	}

	if err = writeIPsToFile(reservedIPs); err != nil {
		return err
	}

	return nil
}

// removeIP removes the given IP from the reserved IPs slice.
func removeIP(ip string, reservedIPs []string) bool {
	for i, rip := range reservedIPs {
		if rip == ip {
			reservedIPs = append(reservedIPs[:i], reservedIPs[i+1:]...)
			return true
		}
	}

	return false
}

func GetAllIPs(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	var ips []string
	for ip = ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		tempIPNet := &net.IPNet{IP: ip, Mask: ipnet.Mask}
		ips = append(ips, tempIPNet.String())
	}
	// remove network address and broadcast address
	return ips[1 : len(ips)-1], nil
}

func inc(ip net.IP) {
	ip = ip.To4()
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] > 0 {
			break
		}
	}
}
