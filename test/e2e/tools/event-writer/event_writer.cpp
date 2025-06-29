#include <winsock2.h>
#include <iphlpapi.h>
#include <bpf/libbpf.h>
#include <bpf/bpf.h>

#include <vector>
#include "event_writer.h"
#include <ebpf_api.h>

bpf_object *obj = NULL;
bpf_link* link = NULL;

int
set_filter(struct filter* flt) {
    uint8_t key = 0;
    int map_flt_fd = 0;

    // Attempt to open the pinned map
    map_flt_fd = bpf_obj_get(FILTER_MAP_PIN_PATH);
    if (map_flt_fd < 0) {
        fprintf(stderr, "%s - failed to lookup filter_map\n", __FUNCTION__);
        return 1;
    }
    if (bpf_map_update_elem(map_flt_fd, &key, flt, 0) != 0) {
        fprintf(stderr, "%s - failed to update filter\n", __FUNCTION__);
        return 1;
    }
    return 0;
}

int _pin(const char* pin_path, int fd, bool is_map) {
    if (bpf_obj_get(pin_path) < 0) {
        if (bpf_obj_pin(fd, pin_path) < 0) {
            fprintf(stderr, "%s - failed to pin %s to %s\n", __FUNCTION__,
                    is_map ? "map" : "prog", pin_path);
            return 1;
        }

        printf("%s - %s successfully pinned at %s\n", __FUNCTION__,
                    is_map ? "map" : "prog",
                    pin_path);
    } else {
        printf("%s - pinned %s found %s\n", __FUNCTION__,
                                            is_map ? "map" : "prog",
                                            pin_path);
    }
    return 0;
}

int
attach_program_to_interface(int ifindx) {
    int evt_wrt_fd = 0;
    evt_wrt_fd = bpf_obj_get(EVENT_WRITER_PIN_PATH);
    if (evt_wrt_fd < 0) {
        fprintf(stderr, "%s - failed to lookup event_writer at %s\n", __FUNCTION__, EVENT_WRITER_PIN_PATH);
        return 1;
    }

    // Verify there's no program attached to the specified ifindex.
    uint32_t program_id;
    if (bpf_xdp_query_id(ifindx, 0, &program_id) < 0) {
        if (bpf_xdp_attach(ifindx, evt_wrt_fd, 0, nullptr) != 0) {
            fprintf(stderr, "%s - failed to attach to interface with ifindex %d\n", __FUNCTION__, ifindx);
            return 1;
        }
        printf("%s - Attached program %s to interface with ifindex %d\n", __FUNCTION__, EVENT_WRITER_PIN_PATH, ifindx);
    } else {
        if (program_id == evt_wrt_fd) {
            printf("%s - program alteady attached %s to interface with ifindex %d\n", __FUNCTION__, EVENT_WRITER_PIN_PATH, ifindx);
        } else {
            if (bpf_xdp_attach(ifindx, evt_wrt_fd, XDP_FLAGS_REPLACE, nullptr) != 0) {
                fprintf(stderr, "%s - failed to attach to interface with ifindex %d\n", __FUNCTION__, ifindx);
                return 1;
            }
        }
    }

    printf("%s - Attached program %s to interface with ifindex %d\n", __FUNCTION__, EVENT_WRITER_PIN_PATH, ifindx);
    return 0;
}

int
load_pin(void) {
    struct bpf_program* prg = NULL;
    struct bpf_map *map_ev = NULL, *map_met = NULL, *map_fvt = NULL, *map_flt = NULL;
    int prg_fd = 0;

    // Load the BPF object file
    obj = bpf_object__open("bpf_event_writer.sys");
    if (obj == NULL) {
        fprintf(stderr, "%s - failed to open BPF object\n", __FUNCTION__);
        goto fail;
    }

    if (EBPF_SUCCESS != ebpf_object_set_execution_type(obj, EBPF_EXECUTION_NATIVE)) {
        fprintf(stderr, "%s - failed to set execution type to native\n", __FUNCTION__);
        goto fail;
    }

    // Load cilium_events map and event_writer bpf  program
    if (bpf_object__load(obj) < 0) {
        fprintf(stderr, "%s - failed to load BPF sys\n", __FUNCTION__);
        goto fail;
    }

    // Find the program by its name
    prg = bpf_object__find_program_by_name(obj, "event_writer");
    if (prg == NULL) {
        fprintf(stderr, "%s - failed to find event_writer program", __FUNCTION__);
        goto fail;
    }

    if (_pin(EVENT_WRITER_PIN_PATH, bpf_program__fd(prg), false) != 0) {
        goto fail;
    }

    // Find the map by its name
    map_ev = bpf_object__find_map_by_name(obj, "cilium_events");
    if (map_ev == NULL) {
        fprintf(stderr, "%s - failed to find cilium_events by name\n", __FUNCTION__);
        goto fail;
    }
    if (_pin(EVENTS_MAP_PIN_PATH, bpf_map__fd(map_ev), true) != 0) {
        goto fail;
    }

    // Find the map by its name
    map_met = bpf_object__find_map_by_name(obj, "cilium_metrics");
    if (map_met == NULL) {
        fprintf(stderr, "%s - failed to find cilium_metrics by name\n", __FUNCTION__);
        goto fail;
    }
    if (_pin(METRICS_MAP_PIN_PATH, bpf_map__fd(map_ev), true) != 0) {
        goto fail;
    }

    // Find the map by its name
    map_fvt = bpf_object__find_map_by_name(obj, "five_tuple_map");
    if (map_fvt == NULL) {
        fprintf(stderr, "%s - failed to find five_tuple_map by name\n", __FUNCTION__);
        goto fail;
    }
    if (_pin(FIVE_TUPLE_MAP_PIN_PATH, bpf_map__fd(map_fvt), true) != 0) {
        goto fail;
    }

    // Find the map by its name
    map_flt = bpf_object__find_map_by_name(obj, "filter_map");
    if (map_flt == NULL) {
        fprintf(stderr, "%s - failed to lookup filter_map\n", __FUNCTION__);
        goto fail;
    }
    if (_pin(FILTER_MAP_PIN_PATH, bpf_map__fd(map_flt), true) != 0) {
        goto fail;
    }

    printf("%s - event-writer loaded successfully\n", __FUNCTION__);
    bpf_object__close(obj);
    return 0; // Return success

fail:
    if (prg != NULL) {
        bpf_program__unpin(prg, EVENT_WRITER_PIN_PATH);
    }

    if (map_ev != NULL) {
        bpf_map__unpin(map_ev, EVENTS_MAP_PIN_PATH);
    }

    if (map_flt != NULL) {
        bpf_map__unpin(map_flt, FILTER_MAP_PIN_PATH);
    }

    if (map_fvt != NULL) {
        bpf_map__unpin(map_fvt, FIVE_TUPLE_MAP_PIN_PATH);
    }

    if (map_met != NULL) {
        bpf_map__unpin(map_met, METRICS_MAP_PIN_PATH);
    }

    if (obj != NULL) {
        bpf_object__close(obj);
    }
    return 1;
}

int
unpin(void) {
    if (bpf_obj_get(EVENT_WRITER_PIN_PATH) < 0) {
        fprintf(stderr, "%s - failed to lookup event_writer at %s\n", __FUNCTION__, EVENT_WRITER_PIN_PATH);
    } else {
        ebpf_object_unpin(EVENT_WRITER_PIN_PATH);
    }

    if (bpf_obj_get(FILTER_MAP_PIN_PATH) < 0) {
        fprintf(stderr, "%s - failed to lookup filter_map at %s\n", __FUNCTION__, FILTER_MAP_PIN_PATH);
   } else {
       ebpf_object_unpin(FILTER_MAP_PIN_PATH);
   }

    if (bpf_obj_get(EVENTS_MAP_PIN_PATH) < 0) {
         fprintf(stderr, "%s - failed to lookup cilium_events at %s\n", __FUNCTION__, EVENTS_MAP_PIN_PATH);
    } else {
        ebpf_object_unpin(EVENTS_MAP_PIN_PATH);
    }

    if (bpf_obj_get(METRICS_MAP_PIN_PATH) < 0) {
        fprintf(stderr, "%s - failed to lookup cilium_metrics at %s\n", __FUNCTION__, METRICS_MAP_PIN_PATH);
    } else {
        ebpf_object_unpin(METRICS_MAP_PIN_PATH);
    }

    if (bpf_obj_get(FIVE_TUPLE_MAP_PIN_PATH) < 0) {
        fprintf(stderr, "%s - failed to lookup five_tuple_map at %s\n", __FUNCTION__, FIVE_TUPLE_MAP_PIN_PATH);
    } else {
        ebpf_object_unpin(FIVE_TUPLE_MAP_PIN_PATH);
    }

    return 0;
 }

uint32_t _ipStrToUint(const char* ipStr) {
    uint32_t ip = 0;
    int part = 0;
    int parts = 0;
    const char *p = ipStr;
    char c;

    while ((c = *p++) != '\0') {
        if (c >= '0' && c <= '9') {
            part = part * 10 + (c - '0');
        } else if (c == '.') {
            ip = (ip << 8) | (part & 0xFF);
            part = 0;
            parts++;
        } else {
            // Invalid character in IP string.
            return 0;
        }
    }

    // Process the last octet.
    ip = (ip << 8) | (part & 0xFF);
    parts++;

    // Ensure we have exactly four parts
    if (parts != 4) {
        return 0;
    }

    return ip;
}

int main(int argc, char* argv[]) {
    setvbuf(stdout, NULL, _IONBF, 0);
    // Parse the command-line arguments (flags)
    if (argc < 2) {
        fprintf(stderr, "valid arguments are required. Exiting..\n");
        return 1;
    }

    if (strcmp(argv[1], "-load-pin") == 0) {
        if (load_pin() != 0) {
            return 1;
        }
    } else if (strcmp(argv[1], "-set-filter") == 0) {
        struct filter flt;
        memset(&flt, 0, sizeof(flt));

        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "-event") == 0) {
                if (i + 1 < argc)
                    flt.event = static_cast<uint8_t>(atoi(argv[++i]));
            } else if (strcmp(argv[i], "-srcIP") == 0) {
                if (i + 1 < argc)
                    flt.srcIP = _ipStrToUint(argv[++i]);
            } else if (strcmp(argv[i], "-dstIP") == 0) {
                if (i + 1 < argc)
                    flt.dstIP = _ipStrToUint(argv[++i]);
            } else if (strcmp(argv[i], "-srcprt") == 0) {
                if (i + 1 < argc)
                    flt.srcprt = static_cast<uint16_t>(atoi(argv[++i]));
            } else if (strcmp(argv[i], "-dstprt") == 0) {
                if (i + 1 < argc)
                    flt.dstprt = static_cast<uint16_t>(atoi(argv[++i]));
            }
        }
        printf("Parsed Values:\n");
        printf("Event: %d\n", flt.event);
        printf("Source IP: %u.%u.%u.%u\n",
               (flt.srcIP >> 24) & 0xFF, (flt.srcIP >> 16) & 0xFF,
               (flt.srcIP >> 8) & 0xFF, flt.srcIP & 0xFF);
        printf("Destination IP: %u.%u.%u.%u\n",
               (flt.dstIP >> 24) & 0xFF, (flt.dstIP >> 16) & 0xFF,
               (flt.dstIP >> 8) & 0xFF, flt.dstIP & 0xFF);
        printf("Source Port: %u\n", flt.srcprt);
        printf("Destination Port: %u\n", flt.dstprt);

        if (set_filter(&flt) != 0) {
            return 1;
        } else {
            printf("filter updated successfully\n");
        }

    } else if (strcmp(argv[1], "-attach") == 0) {
        int ifindx = 0;
        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "-ifindx") == 0) {
                if (i + 1 < argc)
                    ifindx = static_cast<uint16_t>(atoi(argv[++i]));
            }
        }

        printf("Interface Index: %d\n", ifindx);
        if (ifindx <= 0) {
            fprintf(stderr, "valid ifindx is required. Exiting..\n");
            return 1;
        }

        if (attach_program_to_interface(ifindx) != 0) {
            return 1;
        }
    } else if (strcmp(argv[1], "-unpin") == 0) {
        if (unpin() != 0) {
            return 1;
        }
    } else {
        fprintf(stderr, "invalid arguments. Exiting..\n");
        return 1;
    }

    return 0;
}