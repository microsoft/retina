# Plugins

Each metric is associated with a Plugin.
Associated metrics are linked below.
See [Metrics Configuration](../configuration.md) for info on configuration.

| Name                    | Description                                                                                                                  | Metrics in Basic Mode                                  | Metrics in Advanced Mode                                  | Development Guide               |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ | --------------------------------------------------------- | ------------------------------- |
| `packetforward` (Linux) | Counts number of packets/bytes passing through the `eth0` interface of a Node, along with the direction of the packets.      | [Basic Mode](../basic.md#plugin-packetforward-linux)   | Same metrics as Basic mode                                | [Dev Guide](./packetforward.md) |
| `dropreason` (Linux)    | Counts number of packets/bytes dropped on a Node, along with the direction and reason for drop.                              | [Basic Mode](../basic.md#plugin-dropreason-linux)      | [Advanced Mode](../advanced.md#plugin-dropreason-linux)   | [Dev Guide](./dropreason.md)    |
| `linuxutil` (Linux)     | Gathers TCP/UDP statistics and network interface statistics from the `netstats` and `ethtool` Node utilities (respectively). | [Basic Mode](../basic.md#plugin-linuxutil-linux)       | Same metrics as Basic mode                                | [Dev Guide](./linuxutil.md)     |
| `dns` (Linux)           | Counts DNS requests/responses by query, including error codes, response IPs, and other metadata.                             | [Basic Mode](../basic.md#plugin-dns-linux)             | [Advanced Mode](../advanced.md#plugin-dns-linux)          | [Dev Guide](./dns.md)           |
| `hnstats` (Windows)     | Gathers TCP statistics and counts number of packets/bytes forwarded or dropped in HNS and VFP.                               | [Basic Mode](../basic.md#plugin-hnsstats-windows)      | Same metrics as Basic mode                                | [Dev Guide](./hnsstats.md)      |
| `packetparser` (Linux)  | Measures TCP packets passing through `eth0`, providing the ability to calculate TCP-handshake latencies, etc.                | No basic metrics                                       | [Advanced Mode](../advanced.md#plugin-packetparser-linux) | [Dev Guide](./packetparser.md)  |
| `cilium` (Linux) | Collect agent and perf events from cilium via monitor1_2 socket and process flows in our hubble observer | [Metrics](./cilium.md#metrics) | [Metrics](./cilium.md#metrics) | [Dev Guide](./cilium.md) |
