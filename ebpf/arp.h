#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <arpa/inet.h>
#include <bpf/bpf_helpers.h>

struct arphdr {
	__be16		ar_hrd;		/* format of hardware address	*/
	__be16		ar_pro;		/* format of protocol address	*/
	unsigned char	ar_hln;		/* length of hardware address	*/
	unsigned char	ar_pln;		/* length of protocol address	*/
	__be16		ar_op;		/* ARP opcode (command)		*/

	unsigned char		ar_sha[ETH_ALEN];	/* sender hardware address	*/
	unsigned char		ar_sip[4];		/* sender IP address		*/
	unsigned char		ar_tha[ETH_ALEN];	/* target hardware address	*/
	unsigned char		ar_tip[4];		/* target IP address		*/
};

#define	ARPOP_REQUEST	1		/* ARP request			*/
#define	ARPOP_REPLY	2		/* ARP reply			*/

// Sender MAC address of ARP header to be used for the ARP reply(Specific)
unsigned char vxlan_mac[ETH_ALEN] = {0x1a, 0xfb, 0x32, 0x2c, 0x70, 0x33}; // user definition

unsigned char bridge_mac[ETH_ALEN] = {0x96, 0x66, 0xa4, 0x63, 0x8d, 0x17}; // user definition

static __always_inline void swap_mac_addresses(struct __sk_buff *skb) {
  unsigned char src_mac[6];
  unsigned char dst_mac[6];

  bpf_skb_load_bytes(skb, offsetof(struct ethhdr, h_source), src_mac, 6);
  bpf_skb_load_bytes(skb, offsetof(struct ethhdr, h_dest), dst_mac, 6);

  bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_source), vxlan_mac, 6, 0);
  bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_dest), src_mac, 6, 0);
}


#define ARP_SRC_MAC_OFF (ETH_HLEN + offsetof(struct arphdr, ar_sha))
#define ARP_DST_MAC_OFF (ETH_HLEN + offsetof(struct arphdr, ar_tha))



static __always_inline void swap_arp_mac_addresses(struct __sk_buff *skb) {
    unsigned char sender_mac[ETH_ALEN];
    unsigned char target_mac[ETH_ALEN];

    bpf_skb_load_bytes(skb, ARP_SRC_MAC_OFF, sender_mac, ETH_ALEN);
    bpf_skb_load_bytes(skb, ARP_DST_MAC_OFF, target_mac, ETH_ALEN);

    bpf_skb_store_bytes(skb, ARP_SRC_MAC_OFF, vxlan_mac, ETH_ALEN, 0);
    bpf_skb_store_bytes(skb, ARP_DST_MAC_OFF, sender_mac, ETH_ALEN, 0);
}

#define ARP_SRC_IP_OFF (ETH_HLEN + offsetof(struct arphdr, ar_sip))
#define ARP_DST_IP_OFF (ETH_HLEN + offsetof(struct arphdr, ar_tip))



static __always_inline void swap_arp_ip_addresses(struct __sk_buff *skb) {
    unsigned char sender_ip[4];
    unsigned char target_ip[4];
    bpf_skb_load_bytes(skb, ARP_SRC_IP_OFF, sender_ip, 4);
    bpf_skb_load_bytes(skb, ARP_DST_IP_OFF, target_ip, 4);
    bpf_skb_store_bytes(skb, ARP_SRC_IP_OFF, target_ip, 4, 0);
    bpf_skb_store_bytes(skb, ARP_DST_IP_OFF, sender_ip, 4, 0);
}

#define ARP_OP_OFF (ETH_HLEN + offsetof(struct arphdr, ar_op))

static __always_inline void update_arp_op(struct __sk_buff *skb, unsigned short new_op) {
      new_op = htons(new_op);
      bpf_skb_store_bytes(skb, ARP_OP_OFF, &new_op, sizeof(new_op), 0);
}


