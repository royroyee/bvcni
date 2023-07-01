package backend

import (
	"fmt"
	"github.com/pkg/errors"
	pkg "github.com/royroyee/bvcni/pkg/k8s"
	"github.com/royroyee/bvcni/pkg/utils"
	"github.com/vishvananda/netlink"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"net"
	"strings"
	"syscall"
)

const (
	vxlanName     = "vxlan.1"
	vxlanVni      = 1
	vxlanPort     = 8472
	encapOverhead = 50

	bvcniVtepMacAnnotationKey = "bvcni.vtep.mac"
	bvcniHostIPAnnotationKey  = "bvcni.host.ip"
)

func InitVxlan(podCidr string) (*netlink.Vxlan, net.IP, error) {

	// 1. Create VXLAN interface
	vxlanDevice, err := createVxlan()
	if err != nil {
		return nil, nil, errors.Wrap(err, "Faild to create VXLAN interface")
	}

	// 2. Allocate IP address and set up interface
	vxlanDevice, vxlanAddr, err := setVxlan(podCidr, vxlanDevice)

	return vxlanDevice, vxlanAddr, nil
}

// If there is no vxlan interface, create the interface.
func createVxlan() (*netlink.Vxlan, error) {

	gateway, err := getDefaultGatewayInterface()
	if err != nil {
		return nil, errors.Wrap(err, "getDefaultGatewayInterface error")
	}

	localHostAddrs, err := getIfaceAddr(gateway)
	if err != nil {
		return nil, errors.Wrap(err, "getIfaceAddr error")
	}

	if len(localHostAddrs) == 0 {
		return nil, errors.Errorf("length of local host addrs is 0")
	}

	vxlanDevice, err := ensureVxlanExists(gateway, localHostAddrs[0].IP)

	return vxlanDevice, nil
}

func ensureVxlanExists(gateway *net.Interface, srcAddr net.IP) (*netlink.Vxlan, error) {
	link, err := netlink.LinkByName(vxlanName)
	if err != nil {
		if strings.Contains(err.Error(), "Link not found") {
			klog.Infof("vxlan device %s not found, and create it", vxlanName)

			vxlan := &netlink.Vxlan{
				LinkAttrs: netlink.LinkAttrs{
					Name: vxlanName,
					MTU:  gateway.MTU - encapOverhead,
				},
				VxlanId:  vxlanVni,
				Port:     vxlanPort,
				SrcAddr:  srcAddr,
				Learning: false,
				UDPCSum:  true,
				Proxy:    false,
			}

			if err = netlink.LinkAdd(vxlan); err != nil {
				return nil, errors.Wrap(err, "LinkAdd vxlan error")
			}

			return vxlan, nil
		}

		return nil, errors.Wrapf(err, "get link %s error", vxlanName)
	}

	v, ok := link.(*netlink.Vxlan)
	if ok {
		klog.Infof("vxlan device %s already exists", vxlanName)
		return v, nil
	}

	return nil, errors.Errorf("link %s already exists but not vxlan device", vxlanName)
}

func setVxlan(podCidr string, vxlanDevice *netlink.Vxlan) (*netlink.Vxlan, net.IP, error) {
	_, ipnet, err := net.ParseCIDR(podCidr)
	if err != nil {
		return nil, nil, fmt.Errorf("ParseCIDR error: %w", err)
	}

	addrList, err := netlink.AddrList(vxlanDevice, syscall.AF_INET) // AF_INET : 2 , IPv4
	if err != nil {
		return nil, nil, fmt.Errorf("AddrList error: %w", err)
	}

	if len(addrList) == 0 {
		klog.Infof("config vxlan device %s ip: %s", vxlanDevice.Name, ipnet.IP)
		addr := &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   ipnet.IP,
				Mask: net.IPv4Mask(255, 255, 255, 255),
			},
		}

		if err = netlink.AddrAdd(vxlanDevice, addr); err != nil {
			return nil, nil, fmt.Errorf("AddrAdd error: %w", err)
		}
	}

	if err = netlink.LinkSetUp(vxlanDevice); err != nil {
		return nil, nil, fmt.Errorf("LinkSetUp error: %w", err)
	}

	// Make a copy of ipnet.IP
	ip := make(net.IP, len(ipnet.IP))
	copy(ip, ipnet.IP)

	// Reset the last octet to 0. ex) 10.244.1.0 -> 10.244.0.0
	ip[2] = 0

	// Construct new IPNet. ex) 10.244.0.0 -> 10.244.0.0/24
	defaultCidr := &net.IPNet{
		IP:   ip,
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}

	// add routing table
	//	if err = utils.ReplaceRoute(vxlanDevice.Attrs().Index, defaultCidr, ipnet.IP); err != nil {
	if err = utils.ReplaceRoute(vxlanDevice.Attrs().Index, defaultCidr); err != nil {
		klog.Errorf("Error replacing route for vxlan, err : %s, defaultCidr : %s", err, defaultCidr)
		return nil, nil, errors.Wrapf(err, "vxlan add event ReplaceRoute error")
	}
	klog.Infof("ReplaceRoute: ip route add %s via %s dev %s onlink", ipnet.String(), ipnet.IP, vxlanDevice.Name)
	return vxlanDevice, ipnet.IP, nil
}

// TODO - Use a more efficient method instead of annotations
// Original code : https://github.com/nuczzz/mycni
func StoreVxlanInfo(node *coreV1.Node, vxlanDevice *netlink.Vxlan) error {
	newNode := node.DeepCopy()
	newNode.Annotations[bvcniVtepMacAnnotationKey] = vxlanDevice.HardwareAddr.String()
	newNode.Annotations[bvcniHostIPAnnotationKey] = vxlanDevice.SrcAddr.String()

	klog.Infof("mac: %s", vxlanDevice.HardwareAddr.String())
	klog.Infof("host ip: %s", vxlanDevice.SrcAddr.String())

	return pkg.PatchNode(node, newNode)
}

func getDefaultGatewayInterface() (*net.Interface, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return nil, errors.Wrap(err, "RouteList error")
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			if route.LinkIndex <= 0 {
				return nil, errors.Errorf("found default route but could not determine interface")
			}
			return net.InterfaceByIndex(route.LinkIndex)
		}
	}

	return nil, errors.Errorf("unable to find default route")
}

func getIfaceAddr(iface *net.Interface) ([]netlink.Addr, error) {
	return netlink.AddrList(&netlink.Device{
		LinkAttrs: netlink.LinkAttrs{
			Index: iface.Index,
		},
	}, syscall.AF_INET)
}
