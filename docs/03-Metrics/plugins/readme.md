# Plugins

Each metric is associated with a Plugin.
Associated metrics are linked below.
See [Metrics Configuration](../configuration.md) for info on configuration.

To run Retina without any plugins, the `CAP_BPF` capability (since Linux 5.8) is required for memory locking and for loading/using BPF programs. This capability is mandatory. If you're using any plugins, ensure that the necessary capabilities for those plugins are also added. If a plugin requires `CAP_SYS_ADMIN`, you can substitute it for `CAP_BPF`.

| Name                    | Description                                                                                                                  | Metrics in Basic Mode                                  | Metrics in Advanced Mode                                  | Development Guide               |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ | --------------------------------------------------------- | ------------------------------- |
| `packetforward` (Linux) | Counts number of packets/bytes passing through the `eth0` interface of a Node, along with the direction of the packets.      | [Basic Mode](../modes/basic.md#plugin-packetforward-linux)   | Same metrics as Basic mode                                | [Dev Guide](./Linux/packetforward.md) |
| `dropreason` (Linux)    | Counts number of packets/bytes dropped on a Node, along with the direction and reason for drop.                              | [Basic Mode](../modes/basic.md#plugin-dropreason-linux)      | [Advanced Mode](../modes/advanced.md#plugin-dropreason-linux)   | [Dev Guide](./Linux/dropreason.md)    |
| `linuxutil` (Linux)     | Gathers TCP/UDP statistics and network interface statistics from the `netstats` and `ethtool` Node utilities (respectively). | [Basic Mode](../modes/basic.md#plugin-linuxutil-linux)       | Same metrics as Basic mode                                | [Dev Guide](./Linux/linuxutil.md)     |
| `dns` (Linux)           | Counts DNS requests/responses by query, including error codes, response IPs, and other metadata.                             | [Basic Mode](../modes/basic.md#plugin-dns-linux)             | [Advanced Mode](../modes/advanced.md#plugin-dns-linux)          | [Dev Guide](./Linux/dns.md)           |
| `hnstats` (Windows)     | Gathers TCP statistics and counts number of packets/bytes forwarded or dropped in HNS and VFP.                               | [Basic Mode](../modes/basic.md#plugin-hnsstats-windows)      | Same metrics as Basic mode                                | [Dev Guide](./Windows/hnsstats.md)      |
| `packetparser` (Linux)  | Captures TCP and UDP packets traveling to and from pods and nodes.                | No basic metrics                                       | [Advanced Mode](../modes/advanced.md#plugin-packetparser-linux) | [Dev Guide](./Linux/packetparser.md)  |
| `cilium` (Linux) | Collect agent and perf events from cilium via monitor1_2 socket and process flows in our hubble observer | [Metrics](./Linux/ciliumeventobserver.md#metrics) | Same metrics as Basic mode | [Dev Guide](./Linux/ciliumeventobserver.md) |
