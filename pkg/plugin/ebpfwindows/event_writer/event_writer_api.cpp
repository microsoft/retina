#include <bpf/libbpf.h>
#include <bpf/bpf.h>
#include <windows.h>
#include <vector>
#include "event_writer.h"
#include "event_writer_util.h"

std::vector<std::pair<int, struct bpf_link*>> link_list;
bpf_object* obj = NULL;

extern "C" __declspec(dllexport) DWORD
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

extern "C" __declspec(dllexport)  DWORD
check_five_tuple_exists(struct five_tuple* fvt) {
    int map_evt_req_fd;
    int value = 0;

    map_evt_req_fd = bpf_obj_get(FIVE_TUPLE_MAP_PIN_PATH);
    if (map_evt_req_fd < 0) {
        return 1;
    }
    if (bpf_map_lookup_elem(map_evt_req_fd, fvt, &value) != 0) {
        return 1;
    }

    return 0;
}

extern "C" __declspec(dllexport)  DWORD
attach_program_to_interface(int ifindx) {
    struct bpf_program* prg = bpf_object__find_program_by_name(obj, "event_writer");
    bpf_link* link = NULL;
    if (prg == NULL) {
        fprintf(stderr, "%s - failed to find event_writer program", __FUNCTION__);
        return 1;
    }

    link = bpf_program__attach_xdp(prg, ifindx);
    if (link == NULL) {
        fprintf(stderr, "%s - failed to attach to interface with ifindex %d\n", __FUNCTION__, ifindx);
        return 1;
    }

    link_list.push_back(std::pair<int, bpf_link*>{ifindx, link});
    return 0;
}

extern "C" __declspec(dllexport)  DWORD
pin_maps_load_programs(void) {
    struct bpf_program* prg = NULL;
    struct bpf_map *map_ev, *map_met, *map_fvt, *map_flt;
    struct filter flt;
    // Load the BPF object file
    obj = bpf_object__open("bpf_event_writer.sys");
    if (obj == NULL) {
        fprintf(stderr, "%s - failed to open BPF sys file\n", __FUNCTION__);
        return 1;
    }

    // Load cilium_events map and tcp_connect bpf program
    if (bpf_object__load(obj) < 0) {
        fprintf(stderr, "%s - failed to load BPF sys\n", __FUNCTION__);
        bpf_object__close(obj);
        return 1;
    }

    // Find the map by its name
    map_ev = bpf_object__find_map_by_name(obj, "cilium_events");
    if (map_ev == NULL) {
        fprintf(stderr, "%s - failed to find cilium_events by name\n", __FUNCTION__);
        bpf_object__close(obj);
        return 1;
    }
    if (pin_map(EVENTS_MAP_PIN_PATH, map_ev) != 0) {
        return 1;
    }

    // Find the map by its name
    map_met = bpf_object__find_map_by_name(obj, "cilium_metrics");
    if (map_met == NULL) {
        fprintf(stderr, "%s - failed to find cilium_metrics by name\n", __FUNCTION__);
        bpf_object__close(obj);
        return 1;
    }
    if (pin_map(METRICS_MAP_PIN_PATH, map_ev) != 0) {
        return 1;
    }

    // Find the map by its name
    map_fvt = bpf_object__find_map_by_name(obj, "five_tuple_map");
    if (map_fvt == NULL) {
        fprintf(stderr, "%s - failed to find five_tuple_map by name\n", __FUNCTION__);
        bpf_object__close(obj);
        return 1;
    }
    if (pin_map(FIVE_TUPLE_MAP_PIN_PATH, map_fvt) != 0) {
        return 1;
    }

    // Find the map by its name
    map_flt = bpf_object__find_map_by_name(obj, "filter_map");
    if (map_flt == NULL) {
        fprintf(stderr, "%s - failed to lookup filter_map\n", __FUNCTION__);
        return 1;
    }
    if (pin_map(FILTER_MAP_PIN_PATH, map_flt) != 0) {
        return 1;
    }

    memset(&flt, 0, sizeof(flt));
    flt.event = 4; // TRACE
    if (set_filter(&flt) != 0) {
        return 1;
    }

    return 0; // Return success
}

// Function to unload programs and detach
extern "C" __declspec(dllexport) DWORD
unload_programs_detach() {
    for (auto it = link_list.begin(); it != link_list.end(); it ++) {
        auto ifidx = it->first;
        auto link = it->second;
        auto link_fd = bpf_link__fd(link);
        if (bpf_link_detach(link_fd) != 0) {
            fprintf(stderr, "%s - failed to detach link %d\n", __FUNCTION__, ifidx);
        }
        if (bpf_link__destroy(link) != 0) {
            fprintf(stderr, "%s - failed to destroy link %d", __FUNCTION__, ifidx);
        }
    }

    if (obj != NULL) {
        bpf_object__close(obj);
    }

    return 0;
}
