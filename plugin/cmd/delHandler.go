package cmd

import (
	"fmt"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/ns"
	ipa "github.com/royroyee/bvcni/pkg/ip"
	"github.com/royroyee/bvcni/pkg/log"
	"github.com/vishvananda/netlink"
)

func CmdDel(args *skel.CmdArgs) error {
	//// debug
	log.Debugf("cmdDel details: containerID = %s, netNs = %s, ifName = %s, args = %s, path = %s, stdin = %s",
		args.ContainerID,
		args.Netns,
		args.IfName,
		args.Args,
		args.Path,
		string(args.StdinData),
	)

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return err
	}

	defer netns.Close()

	ip, err := fetchIPFromVethInNS(netns, args.IfName)
	if err != nil {
		return err
	}

	err = ipa.ReturnIP(ip)
	if err != nil {
		return err
	}
	return nil
}

// return the IP address for the ifName in container Namespace.
func fetchIPFromVethInNS(netns ns.NetNS, ifName string) (string, error) {
	var ip string
	err := netns.Do(func(_ ns.NetNS) error {
		// Fetch the veth interface.
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to lookup veth %q in %q: %v", ifName, netns.Path(), err)
		}

		// Check if the interface is of type veth.
		if _, ok := link.(*netlink.Veth); !ok {
			return fmt.Errorf("link %s already exists but is not a veth type", ifName)
		}

		// List all addresses associated with the veth.
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("failed to list address for veth %q: %v", ifName, err)
		}

		// Ensure that there is only one address.
		if len(addrs) != 1 {
			return fmt.Errorf("unexpected number of addresses for veth %q: %v", ifName, len(addrs))
		}

		// Save the IP address.
		ip = addrs[0].IPNet.String()
		return nil
	})

	// Return the fetched IP address, if any.
	return ip, err
}
