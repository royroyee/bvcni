#include <linux/types.h>
#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <arpa/inet.h>
#include "arp.h"




__attribute__((section("ingress"), used))
int arp_reply(struct __sk_buff *skb) {

    const int l3_off = ETH_HLEN;                      // IP header offset
    const int l4_off = l3_off + sizeof(struct iphdr); // L4 header offset


    void* data = (void*)(long)skb->data;
    void* data_end = (void*)(long)skb->data_end;

    if (data_end < data + l4_off)
        return TC_ACT_OK;

    struct ethhdr* eth = data;
    struct arphdr* arp = data + sizeof(*eth);


    // Check if the packet is an ARP request
     if (eth->h_proto == htons(ETH_P_ARP)) {

        if (ntohs(arp->ar_op) == ARPOP_REQUEST) {

            // swap mac address(eth)
            swap_mac_addresses(skb);

            // swap mac address(arp header)
            swap_arp_mac_addresses(skb);

            // swap ip address(arp header)
            swap_arp_ip_addresses(skb);

            // Change the type of the ARP packet to ARP_REPLY
            update_arp_op(skb, ARPOP_REPLY);

            // Redirect the reply packet
            bpf_clone_redirect(skb, skb->ifindex, 0);

            // Drop the original request packet
            return TC_ACT_SHOT;
         }
    }
    return TC_ACT_UNSPEC;
}



