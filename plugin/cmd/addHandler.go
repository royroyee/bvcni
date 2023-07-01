package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/royroyee/bvcni/pkg/bridge"
	"github.com/royroyee/bvcni/pkg/config"
	ipa "github.com/royroyee/bvcni/pkg/ip"
	log "github.com/royroyee/bvcni/pkg/log"
	"github.com/vishvananda/netlink"
	"net"
)

const (
	mtu = 1500
)

func CmdAdd(args *skel.CmdArgs) error {

	//// debug
	log.Debugf("cmdAdd details: containerID = %s, netNs = %s, ifName = %s, args = %s, path = %s, stdin = %s",
		args.ContainerID,
		args.Netns,
		args.IfName,
		args.Args,
		args.Path,
		string(args.StdinData),
	)

	// Load CNI config file (/etc/cni/net.d)
	CNIConfig, err := config.LoadCNIConfig(args.StdinData)
	if err != nil {
		log.Debugf("LoadCNIConfig Error : %s", err.Error())
		return err
	}

	// Check if there is a bridge, and if it exists, update it; otherwise, create one.
	br, err := bridge.SetUpBridge(CNIConfig.PodCidr)

	if err != nil || br == nil {
		log.Debugf("Check Bridge Error : %s", err.Error())
	}

	// obtain the pod IP and gateway IP addresses from the pod CIDR. During this process
	// read from and write to the "tmp/reserved_ips" file, ensuring that the IP addresses do not overlap. (This approach is a very basic and simple method; a better approach would be to use the etcd-ipam method.)
	podIP, gwIP, err := ipa.AllocateIPs(CNIConfig.PodCidr)
	if err != nil {
		log.Debugf("Failed to process IPs: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		log.Debugf("GetNS Error %s ", err.Error())
		return err

	}

	defer netns.Close()

	if err = setUpVeth(netns, br, mtu, args.IfName, podIP, gwIP); err != nil {
		log.Debugf("SetUpVethTest error")
		return err
	}

	podIPAddr, _, err := net.ParseCIDR(podIP)
	if err != nil {
		log.Debugf("Invalid pod IP address: %s", podIP)
	}

	gwIPNet, _, err := net.ParseCIDR(gwIP)
	if err != nil {
		log.Debugf("Invalid gateway IP address: %s", gwIP)
	}

	result := &current.Result{
		CNIVersion: "0.3.1",
		IPs: []*current.IPConfig{
			{
				Address: net.IPNet{
					IP:   podIPAddr,
					Mask: net.CIDRMask(24, 32), // 255.255.255.0
				},
				Gateway: gwIPNet,
			},
		},
	}

	resultBytes, err := json.Marshal(result)
	log.Debugf("CmdAdd completion : %s", string(resultBytes))

	return types.PrintResult(result, CNIConfig.CNIVersion)
}

// Code Refer : https://github.com/qingwave/mycni/blob/main/pkg/bridge/bridge.go#L48
// Create a pair of container and host veth interfaces, assign an IP address to the container interface, and connect the host interface to a bridge.
// These veth pairs should be manipulated within their respective namespaces.
// Here, bvcni follows an approach where we create a veth pair in the container network namespace and move one end to the host network namespace.
// Conversely, it is also possible to create a veth pair in the host network namespace and move one end to the container.
func setUpVeth(netns ns.NetNS, br netlink.Link, mtu int, ifName string, podIP string, gatewayIpaddr string) error {
	hostIface := &current.Interface{}
	// Set up the veth interface inside the container network namespace.
	err := netns.Do(func(hostNS ns.NetNS) error {
		// Create the veth pair in the container and move host end into host netns.
		hostVeth, containerVeth, err := ip.SetupVeth(ifName, mtu, "", hostNS)
		if err != nil {
			return fmt.Errorf("failed to setup veth with ifName %q: %w", ifName, err)
		}
		hostIface.Name = hostVeth.Name

		// Get the link for the container veth.
		conLink, err := netlink.LinkByName(containerVeth.Name)
		if err != nil {
			return fmt.Errorf("failed to get link by name %q: %w", containerVeth.Name, err)
		}

		// Parse the pod IP.
		ipaddr, ipnet, err := net.ParseCIDR(podIP)
		if err != nil {
			return fmt.Errorf("failed to parse pod IP %q: %w", podIP, err)
		}
		ipnet.IP = ipaddr

		// Add the address to the container link.
		if err = netlink.AddrAdd(conLink, &netlink.Addr{IPNet: ipnet}); err != nil {
			return fmt.Errorf("failed to add address %q: %w", ipnet, err)
		}

		// Set up the container link.
		if err = netlink.LinkSetUp(conLink); err != nil {
			return fmt.Errorf("failed to setup link %q: %w", conLink, err)
		}

		// Parse the gateway IP address.
		gateway, _, err := net.ParseCIDR(gatewayIpaddr)
		if err != nil {
			return fmt.Errorf("failed to parse gateway IP address %q: %w", gatewayIpaddr, err)
		}

		// Add the default route in the container. ( ex. ip netns exec ns1 ip route add default via 10.244.2.1 )
		if err = ip.AddDefaultRoute(gateway, conLink); err != nil {
			return fmt.Errorf("failed to add default route with gateway %q: %w", gateway, err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to set up veth in netns: %w", err)
	}

	// Lookup the host veth as its index may have changed during ns move.
	hostVeth, err := netlink.LinkByName(hostIface.Name)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %w", hostIface.Name, err)
	}

	if hostVeth == nil {
		return fmt.Errorf("host veth is nil")
	}

	// Connect host veth end to the bridge.
	if err = netlink.LinkSetMaster(hostVeth, br); err != nil {
		return fmt.Errorf("failed to connect %q to bridge %v: %w", hostVeth.Attrs().Name, br.Attrs().Name, err)
	}
	return nil
}
