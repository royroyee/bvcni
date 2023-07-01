package utils

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"net"
	"syscall"
)

func AddArp(vtepDeviceIndex int, vtepIP net.IP, vtepMAC net.HardwareAddr) error {
	return netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    vtepDeviceIndex,
		State:        netlink.NUD_PERMANENT,
		Type:         syscall.RTN_UNICAST,
		IP:           vtepIP,
		HardwareAddr: vtepMAC,
	})
}

func DelArp(vtepDeviceIndex int, vtepIP net.IP, vtepMAC net.HardwareAddr) error {
	return netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    vtepDeviceIndex,
		State:        netlink.NUD_PERMANENT,
		Type:         syscall.RTN_UNICAST,
		IP:           vtepIP,
		HardwareAddr: vtepMAC,
	})
}

func AddFDB(vtepDeviceIndex int, vtepIP net.IP, vtepMAC net.HardwareAddr) error {
	return netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    vtepDeviceIndex,
		Family:       syscall.AF_BRIDGE,
		State:        netlink.NUD_PERMANENT,
		Flags:        netlink.NTF_SELF,
		IP:           vtepIP,
		HardwareAddr: vtepMAC,
	})
}

func DelFDB(vtepDeviceIndex int, vtepIP net.IP, vtepMAC net.HardwareAddr) error {
	return netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    vtepDeviceIndex,
		Family:       syscall.AF_BRIDGE,
		State:        netlink.NUD_PERMANENT,
		Flags:        netlink.NTF_SELF,
		IP:           vtepIP,
		HardwareAddr: vtepMAC,
	})
}

func AddRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       ipn,
		Gw:        gw,
	})
}

// AddDefaultRoute sets the default route on the given gateway.
func AddDefaultRoute(gw net.IP, dev netlink.Link) error {
	_, defNet, _ := net.ParseCIDR("0.0.0.0/0")
	return AddRoute(defNet, gw, dev)
}

func ReplaceRoute(localVtepID int, dst *net.IPNet) error {

	// Check if localVtepID is 0
	if localVtepID == 0 {
		return fmt.Errorf("ReplaceRoute: localVtepID is 0")
	}

	// Check if dst is nil
	if dst == nil {
		return fmt.Errorf("ReplaceRoute: dst is nil")
	}

	// Check if dst.IP is a valid IP address
	if dst.IP == nil || dst.IP.IsUnspecified() {
		return fmt.Errorf("ReplaceRoute: dst.IP is invalid")
	}

	return netlink.RouteReplace(&netlink.Route{
		LinkIndex: localVtepID,
		Dst:       dst,
	})
}

func DelRoute(vtepDeviceIndex int, dst *net.IPNet, gateway net.IP) error {
	return netlink.RouteDel(&netlink.Route{
		LinkIndex: vtepDeviceIndex,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       dst,
		Gw:        gateway,
		Flags:     syscall.RTNH_F_ONLINK,
	})
}
