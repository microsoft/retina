# Data Aggregation

Retina's data aggregation settings are designed to manage the metric volume produced by its agent. The settings allow users to control the number of Flows or events generated. At a higher aggregation level, fewer metrics are produced, which provides a broad overview and ensures resource efficiency in large clusters. Conversely, a lower level of aggregation results in more metrics being generated, offering detailed information such as packets at different points in the Linux kernel.The operational behaviors of Retina at each aggregation level are detailed in the table below:
| Level  | Description|
|---    |---   |
| `low` | `packetparser` will attach a bpf program to the node's default interface, which will help capture metrics for `TO_NETWORK` and `FROM_NETWORK` packets. This will give users a more granular view of packet flows and offers more reliable apiserver latency metrics. |
| `high` | `packetparser` will not attach a bpf program to the node's default interface. As a result, packet observation at this location will be disabled, leading to a reduction in metrics being generated. This configuration is recommended when scalability is the primary concern. However, it is important to note that, due to the absence of packet observation at the default interface, the apiserver latency metrics may not be as reliable. |
