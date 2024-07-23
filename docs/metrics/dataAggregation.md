# Data Aggregation

Retina's data aggregation setting controls the metric volume produced by its agent. A lower aggregation level yields fewer metrics, and a higher level increases them. The following table details Retina's operational behaviors at each aggregation level.

| Level 	| Description 	|
|---	|---	|
| Low 	| `packetparser` will attach a bpf program to the node's default interface, which will help capture metrics for `TO_NETWORK` and `FROM_NETWORK` packets. This will give users a more granular view of packet flows and offers more reliable apiserver latency metrics. 	|
| High 	| `packetparser` will not attach a bpf program to the node's default interface. As a result, packet observation at this location will be disabled, leading to a reduction in metrics being generated. This configuration is recommended when scalability is the primary concern. However, it is important to note that, due to the absence of packet observation at the default interface, the apiserver latency metrics may not be as reliable. 	|