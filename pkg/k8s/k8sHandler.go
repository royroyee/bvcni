package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/royroyee/bvcni/pkg/utils"
	"github.com/vishvananda/netlink"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"net"
	"os"
)

const (
	bvcniVtepMacAnnotationKey = "bvcni.vtep.mac"
	bvcniHostIPAnnotationKey  = "bvcni.host.ip"
)

var (
	clientSet    *kubernetes.Clientset
	config       *rest.Config
	factory      informers.SharedInformerFactory
	nodeInformer cache.SharedIndexInformer
	nodeLister   v1.NodeLister
)

// In Cluster
func InitK8sClient() *kubernetes.Clientset {

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(errors.Wrap(err, "k8s-NewForConfig error"))

	}

	// Create the Kubernetes Client
	clientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(errors.Wrap(err, "k8s-NewForConfig error"))
		// If the Kubernetes client cannot be created, the agent is unable to perform any actions, so it returns a panic.
	}

	return clientSet
}

func InitNodeInformer(clientSet *kubernetes.Clientset, stopCh <-chan struct{}) error {

	factory = informers.NewSharedInformerFactory(clientSet, 0)

	nodeInformer = factory.Core().V1().Nodes().Informer()
	go nodeInformer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, nodeInformer.HasSynced) {
		return errors.Errorf("WaitForCacheSync syncronization error")
	}

	nodeLister = factory.Core().V1().Nodes().Lister()
	return nil
}

func SetUpNodeHandler(nodeName string, vxlan *netlink.Vxlan, vxlanAddr net.IP) {
	filterFunc := func(obj interface{}) bool {

		node, ok := obj.(*coreV1.Node)
		if !ok {
			return false
		}

		if nodeName == node.Name {
			return false
		}
		return true
	}

	addFunc := func(obj interface{}) {
		if err := nodeAddOrUpdate(vxlan, vxlanAddr, obj); err != nil {
			klog.Errorf("nodeAddOrUpdate error %s", err.Error())
		}
	}
	delFunc := func(obj interface{}) {
		if err := nodeDel(vxlan); err != nil {
			klog.Errorf("nodeDel error %s", err)
		}
	}
	updateFunc := func(oldObj, newObj interface{}) {
		oldNode := oldObj.(*coreV1.Node)
		newNode := newObj.(*coreV1.Node)
		if oldNode.Annotations[bvcniVtepMacAnnotationKey] == newNode.Annotations[bvcniVtepMacAnnotationKey] {
			return
		}

		klog.Infof("node update event: %s", newNode.Name)

		if err := nodeAddOrUpdate(vxlan, vxlanAddr, newObj); err != nil {
			klog.Errorf("nodeAddOrUpdate error %s", err.Error())
		}
	}

	nodeInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: filterFunc,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    addFunc,
			UpdateFunc: updateFunc,
			DeleteFunc: delFunc,
		},
	})
}

type NodeData struct {
	IPNet   *net.IPNet
	VtepMac net.HardwareAddr
	HostIP  net.IP
}

func extractNodeData(node *coreV1.Node) (*NodeData, error) {
	_, ipnet, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return nil, fmt.Errorf("unable to parse CIDR %s for node %s: %w", node.Spec.PodCIDR, node.Name, err)
	}

	vtepMac, err := net.ParseMAC(node.Annotations[bvcniVtepMacAnnotationKey])
	if err != nil {
		return nil, fmt.Errorf("unable to parse MAC %s for node %s: %w", node.Annotations[bvcniVtepMacAnnotationKey], node.Name, err)
	}

	hostIP := net.ParseIP(node.Annotations[bvcniHostIPAnnotationKey])
	if hostIP == nil {
		return nil, fmt.Errorf("host IP for node %s is nil", node.Name)
	}

	return &NodeData{
		IPNet:   ipnet,
		VtepMac: vtepMac,
		HostIP:  hostIP,
	}, nil
}

func nodeAddOrUpdate(vxlanDevice *netlink.Vxlan, vxlanAddr net.IP, obj interface{}) error {
	node := obj.(*coreV1.Node)

	data, err := extractNodeData(node)
	if err != nil {
		return err
	}

	if err = utils.AddArp(vxlanDevice.Index, data.IPNet.IP, data.VtepMac); err != nil {
		return fmt.Errorf("error adding ARP for node %s: %w", node.Name, err)
	}

	if err = utils.AddFDB(vxlanDevice.Index, data.HostIP, data.VtepMac); err != nil {
		return fmt.Errorf("error adding FDB for node %s: %w", node.Name, err)
	}

	if err = utils.ReplaceRoute(vxlanDevice.Index, data.IPNet); err != nil {
		return fmt.Errorf("error replacing route for node %s: %w", node.Name, err)
	}

	return nil
}

func nodeDel(vxlanDevice *netlink.Vxlan) func(obj interface{}) {
	return func(obj interface{}) {
		node := obj.(*coreV1.Node)
		data, err := extractNodeData(node)
		if err != nil {
			klog.Errorf("Error extracting data from node %s: %v", node.Name, err)
			return
		}

		klog.Infof("Node delete event: %s", node.Name)

		if err = utils.DelArp(vxlanDevice.Index, data.IPNet.IP, data.VtepMac); err != nil {
			klog.Errorf("Error deleting ARP for node %s: %v", node.Name, err)
			return
		}

		if err = utils.DelFDB(vxlanDevice.Index, data.HostIP, data.VtepMac); err != nil {
			klog.Errorf("Error deleting FDB for node %s: %v", node.Name, err)
			return
		}

		if err = utils.DelRoute(vxlanDevice.Index, data.IPNet, data.IPNet.IP); err != nil {
			klog.Errorf("Error deleting route for node %s: %v", node.Name, err)
			return
		}
	}
}

func GetCurrentNode() (*coreV1.Node, error) {
	nodeName, err := GetCurrentNodeName()
	if err != nil {
		return nil, errors.Wrap(err, "GetCurrentNodeName error")
	}

	node, err := nodeLister.Get(nodeName)
	if err != nil {
		return nil, errors.Wrapf(err, "get node %s error", nodeName)
	}

	return node, nil
}

// Get Node Info (Current Node)
func GetCurrentNodeName() (string, error) {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		podName := os.Getenv("POD_NAME")
		podNamespace := os.Getenv("POD_NAMESPACE")
		if podName == "" || podNamespace == "" {
			return "", errors.Errorf("env POD_NAME and POD_NAMESPACE must be set")
		}

		pod, err := clientSet.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metaV1.GetOptions{})
		if err != nil {
			return "", errors.Wrapf(err, "get pod %s/%s error", podNamespace, podName)
		}

		nodeName = pod.Spec.NodeName
		if podName == "" {
			return "", errors.Errorf("node name not present in pod spec %s/%s", podNamespace, podName)
		}
	}

	return nodeName, nil
}

func PatchNode(oldNode, newNode *coreV1.Node) error {
	oldData, err := json.Marshal(oldNode)
	if err != nil {
		return errors.Wrap(err, "failed to marshal old node")
	}

	newData, err := json.Marshal(newNode)
	if err != nil {
		return errors.Wrap(err, "failed to marshal new node")
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, coreV1.Node{})
	if err != nil {
		return errors.Wrap(err, "failed to create two-way merge patch")
	}

	if _, err = clientSet.CoreV1().Nodes().Patch(context.TODO(), oldNode.Name, types.StrategicMergePatchType,
		patchBytes, metaV1.PatchOptions{}); err != nil {
		return errors.Wrapf(err, "failed to patch node %s", oldNode.Name)
	}

	return nil
}
