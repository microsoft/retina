#include <bpf/libbpf.h>
#include <bpf/bpf.h>
#include <windows.h>
#include <vector>

int pin_map(const char* pin_path, bpf_map* map) {
    int map_fd = 0;
    // Attempt to open the pinned map
    map_fd = bpf_obj_get(pin_path);
    if (map_fd < 0) {
        // Get the file descriptor of the map
        map_fd = bpf_map__fd(map);

        if (map_fd < 0) {
            fprintf(stderr, "%s - failed to get map file descriptor\n", __FUNCTION__);
            return 1;
        }

        if (bpf_obj_pin(map_fd, pin_path) < 0) {
            fprintf(stderr, "%s - failed to pin map to %s\n", pin_path, __FUNCTION__);
            return 1;
        }

        printf("%s - map successfully pinned at %s\n", pin_path, __FUNCTION__);
    } else {
        printf("%s -pinned map found at %s\n", pin_path, __FUNCTION__);
    }
    return 0;
}