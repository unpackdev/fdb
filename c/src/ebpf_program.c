#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/in.h>
#include <linux/types.h>
#include <stdint.h>
#include <bpf/bpf_helpers.h>

#define ETHERNET_MTU 1500  // Ethernet MTU size

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);  // 16MB ring buffer size
} msg_ringbuf SEC(".maps");

SEC("xdp")
int handle_packet(struct xdp_md *ctx) {
    // Get the start and end pointers for the packet data
    unsigned char *data = (unsigned char *)(long)ctx->data;
    unsigned char *data_end = (unsigned char *)(long)ctx->data_end;

    // Calculate the packet length directly from ctx
    uint32_t total_len = data_end - data;

    // Bound the packet length by Ethernet MTU
    if (total_len == 0 || total_len > ETHERNET_MTU)
        return XDP_PASS;  // We pass the packet instead of dropping it

    // Ensure the packet has enough data for the Ethernet header
    if (data + sizeof(struct ethhdr) > data_end)
        return XDP_PASS;  // Not enough data for Ethernet header

    struct ethhdr *eth = (struct ethhdr *)data;

    // Only process IPv4 packets
    if (__constant_htons(eth->h_proto) != ETH_P_IP)
        return XDP_PASS;  // We only handle IPv4, pass the rest

    // Ensure the packet has enough data for the IP header
    struct iphdr *ip = (struct iphdr *)(data + sizeof(struct ethhdr));
    if ((unsigned char *)ip + sizeof(struct iphdr) > data_end)
        return XDP_PASS;  // Not enough data for IP header

    // Check if the packet has enough data for TCP or UDP headers
    unsigned char *transport_header = data + sizeof(struct ethhdr) + sizeof(struct iphdr);
    if (ip->protocol == IPPROTO_TCP && (transport_header + sizeof(struct tcphdr) > data_end))
        return XDP_PASS;  // Not enough data for TCP header
    if (ip->protocol == IPPROTO_UDP && (transport_header + sizeof(struct udphdr) > data_end))
        return XDP_PASS;  // Not enough data for UDP header

    // Reserve space in the ring buffer for the packet data
    void *ringbuf_space = bpf_ringbuf_reserve(&msg_ringbuf, ETHERNET_MTU, 0);
    if (!ringbuf_space)
        return XDP_PASS;  // If we can't reserve space, pass the packet

    // Copy the entire packet into the ring buffer
    if (bpf_probe_read_kernel(ringbuf_space, ETHERNET_MTU, data)) {
        bpf_ringbuf_discard(ringbuf_space, 0);  // Discard the reserved space if reading fails
        return XDP_ABORTED;  // Abort if copying the packet fails
    }

    // Submit the packet to the ring buffer
    bpf_ringbuf_submit(ringbuf_space, 0);

    return XDP_PASS;  // Pass the packet after processing
}

char _license[] SEC("license") = "GPL";
