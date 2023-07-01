package bridge

import (
	"github.com/pkg/errors"
	log "github.com/royroyee/bvcni/pkg/log"
	"github.com/vishvananda/netlink"
	"net"
)

const bridgeName = "cni0"

func addBridgeAddr(podCidr string, bridge *netlink.Bridge) error {

	// Create a bridge address using the podCIDR
	bridgeAddr, err := generateBridgeAddr(podCidr)

	if err != nil {
		return errors.Wrapf(err, "Failed to generate the Gateway IP address")
	}

	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   bridgeAddr.IP,
			Mask: net.IPv4Mask(255, 255, 255, 0), //  /24
		},
	}

	if err = netlink.AddrAdd(bridge, addr); err != nil {
		log.Debugf("addBridgeAddr - AddrAdd error : %s", err.Error())
	}

	// 3. Update the Routing Table
	err = addRoutingTable(podCidr, bridge)
	if err != nil {
		return errors.Wrapf(err, "Faild to add the routing table to the bridge")
	}
	return nil
}

func addRoutingTable(podCidr string, bridge *netlink.Bridge) error {

	_, ipNet, err := net.ParseCIDR(podCidr)
	if err != nil {
		return errors.Wrapf(err, "invalid pod CIDR")
	}

	route := netlink.Route{
		LinkIndex: bridge.Attrs().Index,
		Dst:       ipNet,
	}

	// Add a routing table entry for the specified destination network (ipNet) via the bridge
	if err = netlink.RouteAdd(&route); err != nil {
		return errors.Wrapf(err, "bridge add route error")
	}
	return nil
}

func generateBridgeAddr(podCidr string) (*net.IPNet, error) {

	_, ipNet, err := net.ParseCIDR(podCidr)
	if err != nil {
		return nil, errors.Wrapf(err, "generateBridgeAddr - Failed to parse CIDR")
	}

	// Create a copy of ipNet
	bridgeAddr := *ipNet

	// Increment the last octet of the IP address to generate the gateway address
	bridgeAddr.IP[3]++

	return &bridgeAddr, nil
}

func SetUpBridge(podCidr string) (*netlink.Bridge, error) {

	// Check if the bridge exists and exit if it does.
	link, err := netlink.LinkByName(bridgeName)
	if link != nil {
		return link.(*netlink.Bridge), nil
	}

	// Create the bridge
	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   bridgeName,
			MTU:    1500,
			TxQLen: -1,
		},
	}

	err = netlink.LinkAdd(bridge)
	if err != nil {
		log.Debugf("SetUpBridge LinkAdd Error")
	}

	err = netlink.LinkSetUp(bridge)
	if err != nil {
		log.Debugf("SetUpBridge LinkSetUp Error ")
	}

	err = addBridgeAddr(podCidr, bridge)
	if err != nil {
		log.Debugf("SetUpBridge addBridgeAddr Error")
	}
	return bridge, nil
}
