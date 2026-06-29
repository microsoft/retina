#include <bpf/libbpf.h>
#include <bpf/bpf.h>
#include <windows.h>
#include <vector>

int pin_map(const char* pin_path, bpf_map* map);