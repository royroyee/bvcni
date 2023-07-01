package iptables

import (
	"github.com/coreos/go-iptables/iptables"
	"github.com/pkg/errors"
	"net"
	"os/exec"
)

func UpdateIptables(podCidr string) error {

	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		// If iptables is not found, return an error and exit.
		return errors.Wrapf(err, "Failed to setup IPtables. iptables binary was not found")
	}

	// Add forward rule for the /16 subnet instead of the pod CIDR range for VXLAN
	err = appendForwardRule(ipt, podCidr)
	if err != nil {
		return errors.Wrapf(err, "Failed to add FORWARD rule")
	}

	// Add a rule to the nat POSTROUTING chain to masquerade traffic from the specified source IP range (podCIDR).
	err = appendMasqueradeRule(ipt, podCidr)
	if err != nil {
		return errors.Wrapf(err, "Failed to add MASQUERADE rule")
	}

	// Check IP forwarding and enable if necessary.
	err = enableIPForwarding()
	if err != nil {
		return errors.Wrapf(err, "Failed to enable IP forwarding")
	}

	// Set the FORWARD chain policy.
	err = setForwardChainPolicy()
	if err != nil {
		return errors.Wrapf(err, "Failed to set FORWARD chain policy")
	}

	return nil
}

func appendMasqueradeRule(ipt *iptables.IPTables, podCidr string) error {

	err := ipt.AppendUnique("nat", "POSTROUTING", "-s", podCidr, "-j", "MASQUERADE")
	if err != nil {
		return errors.Wrap(err, "appendMasqueradeRule Error")
	}

	return nil
}

func appendForwardRule(ipt *iptables.IPTables, podCidr string) error {

	_, ipNet, err := net.ParseCIDR(podCidr)
	if err != nil {
		return errors.Wrap(err, "Invalid CIDR")
	}

	maskSize, _ := ipNet.Mask.Size()
	if maskSize == 24 {
		newMask := net.CIDRMask(16, 32)
		ipNet.Mask = newMask
		podCidr = ipNet.String()
	}

	err = ipt.AppendUnique("filter", "FORWARD", "-s", podCidr, "-j", "ACCEPT")
	if err != nil {
		return errors.Wrap(err, "appendForwardSourceRule Error")
	}

	err = ipt.AppendUnique("filter", "FORWARD", "-d", podCidr, "-j", "ACCEPT")
	if err != nil {
		return errors.Wrap(err, "appendForwardDestinationRule Error")
	}

	return nil
}

// enables IP forwarding by executing the sysctl command.
func enableIPForwarding() error {
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "Failed to enable IP forwarding")
	}
	return nil
}

// Set the FORWARD chain policy to ACCEPT using the iptables command.
func setForwardChainPolicy() error {
	cmd := exec.Command("iptables", "--policy", "FORWARD", "ACCEPT")
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "Failed to set FORWARD chain policy")
	}
	return nil
}
