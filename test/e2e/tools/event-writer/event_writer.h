/* Copyright (c) Microsoft Corporation */
/* SPDX-License-Identifier: MIT */

#ifndef _EVENT_WRITER__
#define _EVENT_WRITER__


#define EVENTS_MAP_PIN_PATH \
    "/ebpf/global/cilium_events"

#define METRICS_MAP_PIN_PATH \
    "/ebpf/global/cilium_metrics"

#define WINDOWS_METRICS_MAP_PIN_PATH \
    "/ebpf/global/windows_metrics"

#define FILTER_MAP_PIN_PATH \
    "/ebpf/global/filter_map"

#define FIVE_TUPLE_MAP_PIN_PATH \
    "/ebpf/global/five_tuple_map"

#define EVENT_WRITER_PIN_PATH \
    "/ebpf/global/event_writer"

#define DROP_PKTMON -220
#define Drop_FL_InterfaceNotReady 607

enum {
	CILIUM_NOTIFY_UNSPEC = 0,
	CILIUM_NOTIFY_DROP,
	CILIUM_NOTIFY_DBG_MSG,
	CILIUM_NOTIFY_DBG_CAPTURE,
	CILIUM_NOTIFY_TRACE,
	CILIUM_NOTIFY_POLICY_VERDICT,
	CILIUM_NOTIFY_CAPTURE,
	CILIUM_NOTIFY_TRACE_SOCK,
    PKTMON_NOTIFY_DROP = 100,
};

enum {
	METRIC_INGRESS = 1,
	METRIC_EGRESS,
};

struct ethhdr {
    uint8_t dst_mac[6];
    uint8_t src_mac[6];
    uint16_t ethertype;
};

struct iphdr {
    uint8_t ihl : 4,
            version : 4;
    uint8_t tos;
    uint16_t tot_len;
    uint16_t id;
    uint16_t frag_off;
    uint8_t ttl;
    uint8_t protocol;
    uint16_t check;
    uint32_t saddr;
    uint32_t daddr;
};

struct tcphdr {
    uint16_t source;       // Source port
    uint16_t dest;         // Destination port
    uint32_t seq;          // Sequence number
    uint32_t ack_seq;      // Acknowledgment number
    uint8_t  doff;       // Data offset
    uint8_t  res1:4;       // Reserved
    uint8_t  fin:1,
            syn:1,
            rst:1,
            psh:1,
            ack:1,
            urg:1,
            ece:1,
            cwr:1,
            ns:1;
    uint16_t window;       // Window size
    uint16_t check;        // Checksum
    uint16_t urg_ptr;      // Urgent pointer
};

struct udphdr {
    uint16_t source;   // Source port
    uint16_t dest;     // Destination port
    uint16_t len;      // Length of the UDP packet (header + data)
    uint16_t check;    // Checksum
};

union v6addr {
    struct {
        uint32_t p1;
        uint32_t p2;
        uint32_t p3;
        uint32_t p4;
    };
    struct {
        __u64 d1;
        __u64 d2;
    };
    uint8_t addr[16];
}__packed;

struct five_tuple {
    uint8_t proto;
    uint32_t srcIP;
    uint32_t dstIP;
    uint16_t srcprt;
    uint16_t dstprt;
};

struct filter {
    uint8_t    event;
    uint32_t   srcIP;
    uint32_t   dstIP;
    uint16_t   srcprt;
    uint16_t   dstprt;
};

struct trace_notify {
	uint8_t		type;
    uint8_t		subtype;
	uint16_t		source;
	uint32_t		hash;
    uint32_t		len_orig;
	uint16_t		len_cap;
	uint16_t		version;
	uint32_t		src_label;
	uint32_t		dst_label;
	uint16_t		dst_id;
	uint8_t		reason;
	uint8_t		ipv6:1;
	uint8_t		pad:7;
	uint32_t		ifindex;
	union {
		struct {
			uint32_t		orig_ip4;
			uint32_t		orig_pad1;
			uint32_t		orig_pad2;
			uint32_t		orig_pad3;
		};
		union v6addr	orig_ip6;
	};
	uint8_t        data[128];
};

struct drop_notify {
	uint8_t		type;
    uint8_t		subtype;
	uint16_t		source;
	uint32_t		hash;
    uint32_t		len_orig;
	uint16_t		len_cap;
	uint16_t		version;
	uint32_t		src_label;
	uint32_t		dst_label;
	uint32_t		dst_id; /* 0 for egress */
	uint16_t		line;
	uint8_t		file;
	int8_t		ext_error;
	uint32_t		ifindex;
	uint8_t        data[128];
};

struct metrics_key {
	uint8_t     reason;	/* 0: forwarded, >0 dropped */
	uint8_t     dir:2,	/* 1: ingress 2: egress */
		        pad:6;
	uint16_t	line;		/* __MAGIC_LINE__ */
	uint8_t	    file;		/* __MAGIC_FILE__, needs to fit __source_file_name_to_id */
	uint8_t	    reserved[3];	/* reserved for future extension */
};

struct windows_metrics_key {
    uint8_t  type;
    uint16_t reason; /* 0: forwarded, >0 dropped */
    uint8_t  dir : 2, /* 1: ingress 2: egress */
             pad : 6;
    uint16_t line; /* __MAGIC_LINE__ */
    uint8_t  file;  /* __MAGIC_FILE__, needs to fit __source_file_name_to_id */
};

struct metrics_value {
	uint64_t	count;
	uint64_t	bytes;
};

typedef struct _netevent_data_header
{
    uint8_t type;
    uint16_t version;
} netevent_data_header_t;

#pragma pack(push, 1)

/* packet descriptor used for event streaming */
typedef struct _pktmon_evt_stream_packet_descriptor
{
    uint32_t packet_original_length;
    uint32_t packet_logged_length;
    uint32_t packet_metadata_length;
} pktmon_evt_stream_packet_descriptor;

/* metadata information used for event streaming */
typedef struct _pktmon_evt_stream_metadata
{
    uint64_t pkt_groupid;
    uint16_t pkt_count;
    uint16_t appearance_count;
    uint16_t direction_name;
    uint16_t packet_type;
    uint16_t component_id;
    uint16_t edge_id;
    uint16_t filter_id;
    uint32_t drop_reason;
    uint32_t drop_location;
    uint16_t proc_num;
    uint64_t timestamp;
} pktmon_evt_stream_metadata;

/* packet header used for event streaming */
typedef struct _pktmon_evt_stream_packet_header
{
    uint8_t eventid;
    pktmon_evt_stream_packet_descriptor packet_descriptor;
    pktmon_evt_stream_metadata metadata;
} pktmon_evt_stream_packet_header;

typedef struct pktmon_notify {
    netevent_data_header_t version_header;
    pktmon_evt_stream_packet_header  pktmon_header;
    uint8_t                           data[128];
} pktmon_notify_t;

#pragma pack(pop)

#endif  /* _EVENT_WRITER__ */