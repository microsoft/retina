#include "bpf_helpers.h"
#include "bpf_helper_defs.h"
#include "bpf_endian.h"
#include "xdp/ebpfhook.h"
#include "event_writer.h"

SEC (".maps")
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct five_tuple);
    __type(value, uint8_t);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
    __uint(max_entries, 512 * 4096);
} five_tuple_map;

SEC(".maps")
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, uint8_t);
    __type(value, struct filter);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
    __uint(max_entries, 1);
} filter_map;

SEC(".maps")
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, uint32_t);
    __type(value, struct trace_notify);
    __uint(max_entries, 1);
} trc_buffer;

SEC(".maps")
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, uint32_t);
    __type(value, struct drop_notify);
    __uint(max_entries, 1);
} drp_buffer;

SEC(".maps")
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, uint32_t);
    __type(value, struct pktmon_notify);
    __uint(max_entries, 1);
} pktmon_buffer;

SEC(".maps")
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
    __uint(max_entries, 64 * 1024);
} cilium_events;

SEC(".maps")
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct metrics_key);
	__type(value, struct metrics_value);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
	__uint(max_entries, 512 * 4096);
} cilium_metrics;

SEC(".maps")
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct windows_metrics_key);
	__type(value, struct metrics_value);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
	__uint(max_entries, 512 * 4096);
} windows_metrics;

void update_metrics(uint64_t bytes, uint8_t direction,
					uint8_t reason, uint16_t line, uint8_t file)
{
	struct metrics_value *entry, new_entry = {};
	struct metrics_key key = {};

	key.reason = reason;
	key.dir    = direction;
	key.line   = line;
	key.file   = file;

	entry = bpf_map_lookup_elem(&cilium_metrics, &key);
	if (entry) {
		entry->count += 1;
		entry->bytes += bytes;
	} else {
		new_entry.count = 1;
		new_entry.bytes = bytes;
		bpf_map_update_elem(&cilium_metrics, &key, &new_entry, 0);
	}
}

void create_trace_ntfy_event(struct trace_notify* trc_elm)
{
    memset(trc_elm, 0, sizeof(struct trace_notify));
    trc_elm->type       = CILIUM_NOTIFY_TRACE;
    trc_elm->subtype    = 0;
	trc_elm->source     = 10; // random source
	trc_elm->hash       = 0;
    trc_elm->len_orig   = 128;
	trc_elm->len_cap    = 128;
    trc_elm->version    = 1;
	trc_elm->src_label	= 0;
	trc_elm->dst_label	= 0;
	trc_elm->dst_id		= 0;
	trc_elm->reason		= 0;
	trc_elm->ifindex	= 0;
}

void create_drop_event(struct drop_notify* drp_elm)
{
    memset(drp_elm, 0, sizeof(struct drop_notify));
    drp_elm->type       = CILIUM_NOTIFY_DROP;
	drp_elm->subtype    = 6;
	drp_elm->source     = 10; // random source
	drp_elm->hash       = 0;
	drp_elm->len_orig   = 128;
	drp_elm->len_cap    = 128;
	drp_elm->version    = 1;
	drp_elm->src_label	= 0;
	drp_elm->dst_label	= 0;
	drp_elm->dst_id		= 0;
	drp_elm->line		= 0;
    drp_elm->file		= 0;
    drp_elm->ext_error	= 0;
	drp_elm->ifindex	= 0;
}

void create_pktmon_drop_event(struct pktmon_notify* drp_elm)
{
    memset(drp_elm, 0, sizeof(struct pktmon_notify));
    drp_elm->type       = CILIUM_NOTIFY_DROP;
	drp_elm->subtype    = 6;
	drp_elm->source     = 10; // random source
	drp_elm->hash       = 0;
	drp_elm->len_orig   = 128;
	drp_elm->len_cap    = 128;
	drp_elm->version    = 1;
	drp_elm->src_label	= 0;
	drp_elm->dst_label	= 0;
	drp_elm->dst_id		= 0;
	drp_elm->line		= 0;
    drp_elm->file		= 0;
    drp_elm->ext_error	= 0;
	drp_elm->ifindex	= 0;
}

int
check_filter(struct filter* flt, struct five_tuple* tup) {

    if (flt->srcIP != 0 && flt->srcIP != tup->srcIP) {
        return 1;
    }

    if (flt->dstIP != 0 && flt->dstIP != tup->dstIP) {
        return 1;
    }

    if (flt->srcprt != 0 && flt->srcprt != tup->srcprt) {
        return 1;
    }

    if (flt->dstprt != 0 && flt->dstprt != tup->dstprt) {
        return 1;
    }

    return 0;
}

int extract_five_tuple_info(void* data, int bytes_to_copy, struct five_tuple* tup) {
    struct ethhdr *eth;
    uint8_t present = 1;

    if (bytes_to_copy < sizeof(struct ethhdr)) {
        return 1;
    }

    eth = data;
    if (eth->ethertype != htons(0x0800)) {
        return 1;
    }

    if (bytes_to_copy < sizeof(struct ethhdr) + sizeof(struct iphdr)) {
        return 1;
    }

    struct iphdr *iph = data + sizeof(struct ethhdr);

    // Only process TCP or UDP packets
    if (iph->protocol != 6 && iph->protocol != 17) {
        return 1;
    }

    tup->srcIP = htonl(iph->saddr);
    tup->dstIP = htonl(iph->daddr);
    tup->proto = iph->protocol;

    if (tup->proto == 6) {
        if (bytes_to_copy < sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct tcphdr)) {
            return 1;
        }

        struct tcphdr *tcph = data + sizeof(struct ethhdr) + sizeof(struct iphdr);
        tup->srcprt = htons(tcph->source);
        tup->dstprt = htons(tcph->dest);
    }
    else if (tup->proto == 17) {
        if (bytes_to_copy < sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct udphdr)) {
            return 1;
        }

        struct udphdr *udph = data + sizeof(struct ethhdr) + sizeof(struct iphdr);
        tup->srcprt = htons(udph->source);
        tup->dstprt = htons(udph->dest);
    }
    return 0;
}

SEC("xdp")
int
event_writer(xdp_md_t* ctx) {
    uint8_t flt_key = 0;
    uint32_t buf_key = 0;
    struct filter* flt;
    struct five_tuple tup;
    uint32_t size_to_copy = 128;
    uint8_t flt_evttype, present = 1;
    uint8_t reason  = 0;

    if ((uintptr_t)ctx->data + size_to_copy > (uintptr_t)ctx->data_end) {
		size_to_copy = (uintptr_t)ctx->data_end - (uintptr_t)ctx->data;
	}

    memset(&tup, 0, sizeof(tup));
    if (extract_five_tuple_info(ctx->data, size_to_copy, &tup) != 0) {
        return XDP_PASS;
    }

    flt = (struct filter*) bpf_map_lookup_elem(&filter_map, &flt_key);
	if (flt == NULL) {
		return XDP_PASS;
	}

    if (check_filter(flt, &tup) != 0) {
        return XDP_PASS;
    }

    if (bpf_map_update_elem(&five_tuple_map, &tup, &present, BPF_ANY) != 0) {
        return XDP_PASS;
    }

    flt_evttype = flt->event;
	if (flt_evttype == CILIUM_NOTIFY_TRACE) {
        struct trace_notify* trc_elm;

        //Create a Mock Trace Event
        trc_elm = (struct trace_notify *) bpf_map_lookup_elem(&trc_buffer, &buf_key);
        if (trc_elm == NULL) {
            return XDP_PASS;
        }
        create_trace_ntfy_event(trc_elm);
        memset(trc_elm->data, 0, sizeof(trc_elm->data));
        memcpy(trc_elm->data, ctx->data, size_to_copy);
        bpf_perf_event_output(ctx, &cilium_events, EBPF_MAP_FLAG_CURRENT_CPU , trc_elm, sizeof(struct trace_notify));
    }
    else if (flt_evttype == CILIUM_NOTIFY_DROP) {
        struct drop_notify* drp_elm;

        //Create a Mock Drop Event
        drp_elm = (struct drop_notify *) bpf_map_lookup_elem(&drp_buffer, &buf_key);
        if (drp_elm == NULL) {
            return XDP_PASS;
        }
        reason = 130;
        create_drop_event(drp_elm);
        memset(drp_elm->data, 0, sizeof(drp_elm->data));
        memcpy(drp_elm->data, ctx->data, size_to_copy);
        bpf_perf_event_output(ctx, &cilium_events, EBPF_MAP_FLAG_CURRENT_CPU , drp_elm, sizeof(struct drop_notify));

        // Create Windows specific drop event with hardcoded reason code
        {
            struct metrics_value *win_entry, win_new_entry = {};
            struct windows_metrics_key win_key = {};

            win_key.type   = -DROP_PKTMON;
            win_key.reason = Drop_FL_InterfaceNotReady;
            win_key.dir    = METRIC_INGRESS;
            win_key.line   = 0;
            win_key.file   = 0;

            win_entry = bpf_map_lookup_elem(&windows_metrics, &win_key);
            if (win_entry) {
                win_entry->count += 1;
                win_entry->bytes += size_to_copy;
            } else {
                win_new_entry.count = 1;
                win_new_entry.bytes = size_to_copy;
                bpf_map_update_elem(&windows_metrics, &win_key, &win_new_entry, 0);
            }
        }
    }
    else if (flt_evttype == PKTMON_NOTIFY_DROP) {
        struct pktmon_notify* drp_elm;

        //Create a Mock Drop Event
        drp_elm = (struct pktmon_notify *) bpf_map_lookup_elem(&pktmon_buffer, &buf_key);
        if (drp_elm == NULL) {
            return XDP_PASS;
        }
        reason = 130;
        create_pktmon_drop_event(drp_elm);
        memset(drp_elm->data, 0, sizeof(drp_elm->data));
        memcpy(drp_elm->data, ctx->data, size_to_copy);

        bpf_printk("PKTMON_NOTIFY_DROP event: reason=%d, size_to_copy=%d\n", reason, size_to_copy);
        // memcpy(drp_elm->data, ctx->data, size_to_copy);
        bpf_perf_event_output(ctx, &cilium_events, EBPF_MAP_FLAG_CURRENT_CPU , drp_elm, sizeof(struct pktmon_notify));
    }
    update_metrics(size_to_copy, METRIC_INGRESS, reason, 0, 0);

    return XDP_PASS;
}