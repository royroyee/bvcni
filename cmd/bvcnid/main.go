package main

import (
	"github.com/royroyee/bvcni/pkg/backend"
	"github.com/royroyee/bvcni/pkg/config"
	"github.com/royroyee/bvcni/pkg/iptables"
	pkg "github.com/royroyee/bvcni/pkg/k8s"
	"github.com/royroyee/bvcni/pkg/signals"
	"k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

func main() {
	flag.InitFlags()

	// Run CNI Agent(bvcnid)
	runCNIAgent()
}

func runCNIAgent() {

	stopCh := signals.SetupSignalHandler()

	// Init K8s Client in Cluster
	clientSet := pkg.InitK8sClient()

	// Create NodeInformer
	pkg.InitNodeInformer(clientSet, stopCh)

	// Get Node information (Current Node)
	node, err := pkg.GetCurrentNode()
	if err != nil {
		klog.Fatalf("GetCurrentNode error : %s", err.Error())
	}

	// Init CNI plugin file
	err = config.InitCNIPluginConfigFile(node)
	if err != nil {
		klog.Fatalf("InitCNIPluginConfigFile error : %s", err.Error())
	}

	// Update iptables
	iptables.UpdateIptables(node.Spec.PodCIDR)

	// Create VXLAN interface
	vxlanLink, vxlanAddr, err := backend.InitVxlan(node.Spec.PodCIDR)
	if err != nil {
		klog.Fatalf("Init VXLAN interface error: %s", err.Error())
	}

	// Each node stores its VXLAN information in its own node annotations for updating ARP and FDB
	err = backend.StoreVxlanInfo(node, vxlanLink)
	if err != nil {
		klog.Fatalf("Store VXLAN infro error: %s", err.Error())
	}

	// Add Handler of NodeInformer
	pkg.SetUpNodeHandler(node.Name, vxlanLink, vxlanAddr)
	<-stopCh
}
