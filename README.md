# bvcni
A Simple CNI plugin based on Linux bridge and VXLAN for Kubernetes

## Introduction
**bvcni** is a Container Networking Interface (CNI) plugin designed for use in Kubernetes clusters. The plugin facilitates container networking management by leveraging a combination of Bridge and VXLAN technologies.


## Architecture
bvcni employs Linux-based bridges and VXLAN for its architecture, mimicking the Flannel VXLAN mode approach.


![bvcni architecture.png](img%2Fbvcni%20architecture.png)

## Features
bvcni aims for simplicity, minimizing complex configurations.
- **Bridge Networking**: Connects containers within the same network bridge, enabling communication between them.
- **VXLAN Networking**: Establishes connections between containers using VXLAN virtual networks, ensuring network isolation and scalability.

## Prerequisites
- No other CNI should be installed. If another CNI was previously used, a reset is recommended.

## Build and Test
- [Docker Hub](https://hub.docker.com/r/royroyee/bvcnid)

Apply the following command when no other CNI is installed:




```
kubectl apply -f https://raw.githubusercontent.com/royroyee/bvcni/new_branch/bvcni.yaml
```

> Note Run the command when no other CNI is installed.

Prior to the CNI installation, the node will be in the NOT READY state. Once applied, a DaemonSet named `bvcnid` will be deployed to each node. The CNI configuration file will be generated in the `/etc/cni/net.d` directory, and the plugin binary files will be stored in `/opt/cni/bin`. 
A temporary file, `tmp/reserved_ips`, is also created for IP allocation.


### CNI Config File (00-bvcni.conf)
```
{
  "cniVersion": "0.3.1",
  "name": "bvcni",
  "type": "bvcni",
  "podcidr": "10.244.1.0/24"
}
```

    

Afterwards, the node will transition to the READY state, and you will be able to use kubernetes.

```
NAME                                 READY   STATUS             RESTARTS          AGE
bvcnid-68n69                         1/1     Running               0              10s
bvcnid-hrqzh                         1/1     Running               0              10s
bvcnid-t86df                         1/1     Running               0              10s

NAME          STATUS   ROLES                  AGE     VERSION
k8s-master    Ready    control-plane,master   3d      v1.23.0
k8s-worker1   Ready    worker                 3d      v1.23.0
k8s-worker2   Ready    worker                 2d22h   v1.23.0
```

```
royroyee@k8s-master:~$ kubectl get pods -o wide
NAME      READY   STATUS    RESTARTS      AGE    IP           NODE          NOMINATED NODE   READINESS GATES
alpine1   1/1     Running      0         164m   10.244.2.2   k8s-worker2   <none>           <none>
alpine2   1/1     Running      0         85m    10.244.2.4   k8s-worker2   <none>           <none>
```

### Internet
```
royroyee@k8s-master:~$ kubectl exec -it alpine1 -- ping 8.8.8.8
PING 8.8.8.8 (8.8.8.8): 56 data bytes
64 bytes from 8.8.8.8: seq=0 ttl=52 time=28.315 ms
64 bytes from 8.8.8.8: seq=1 ttl=52 time=28.116 ms
64 bytes from 8.8.8.8: seq=2 ttl=52 time=28.130 ms
```


### Same node container network connection
```
hwan@k8s-master:~$ kubectl exec -it alpine1 -- ping 10.244.2.4
PING 10.244.2.4 (10.244.2.4): 56 data bytes
64 bytes from 10.244.2.4: seq=0 ttl=64 time=0.067 ms
64 bytes from 10.244.2.4: seq=1 ttl=64 time=0.097 ms
```

### Different nodes container network connection
TBF






## Known issues
- There are occasional instances where the VXLAN ARP, FDB, and routing table do not update correctly during DaemonSet build
  - This can be resolved by deleting and re-running the daemonset (i.e., running it twice).


- Communication between nodes is possible, but there are issues with pod-to-pod communication across different nodes.
    - We are currently investigating the issue, and although there is a potential solution using eBPF, it involves complex aspects, so we are currently putting it on hold.


- There is an issue with the proper update of iptables settings.
  - If the iptables update has not occurred or if internet connectivity is not available within the pod, you must manually execute the following command (per current manual instructions):

You need to perform this on each node :
``` 
$ iptables -A FORWARD -s 10.244.0.0/16 -j ACCEPT

$ iptables -A FORWARD -d 10.244.0.0/16 -j ACCEPT 

$ iptables -t nat -A POSTROUTING -s 10.244.2.0/24 -j MASQUERADE
```
`10.244.2.0/24` : node's PODCIDR


    
  
> We plan to fix this issue as soon as possible.

## Contributing
This project is still under development and there are many areas that need improvement. Feedback and suggestions are always welcome through issues or pull requests

## Contributors
[Younghwan Kim](https://github.com/royroyee)

## License
[MIT License](https://github.com/royroyee/bvcni/blob/new_branch/LICENSE)
