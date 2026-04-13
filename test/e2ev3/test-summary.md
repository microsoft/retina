## E2E Test Summary

**Provider:** `kind`

### Workflow Results

| Workflow | Status | Duration | Details |
|---|---|---|---|
| basic-metrics | ✅ passed | 2m21s |  |
| advanced-metrics | ✅ passed | 1m5s |  |
| hubble-metrics | ✅ passed | 4m6s |  |
| basic-metrics-experimental | ✅ passed | 1m19s |  |
| advanced-metrics-experimental | ✅ passed | 1m2s |  |
| capture | ✅ passed | 38s |  |

### Skipped Scenarios

| Workflow | Scenario | Reason |
|---|---|---|
| basic-metrics-experimental | network_stats | host-level counters are zero on Kind nodes |
| basic-metrics-experimental | node_connectivity | host-level counters are zero on Kind nodes |
| advanced-metrics-experimental | adv_drop_count/bytes | dropreason eBPF hooks do not capture drops on Kind |
| advanced-metrics-experimental | adv_tcpretrans_count | retransmissions are near-zero on Kind's local network |

**Total:** 6 passed, 0 failed, 0 skipped workflows | 4 skipped scenarios
