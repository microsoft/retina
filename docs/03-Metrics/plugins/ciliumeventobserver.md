# `ciliumEventObserver` (Linux)

Collect agent and perf events from cilium via monitor1_2 socket. This allows us to serve additional metrics and flows alongside Cilium events.

## Architecture

Cilium collects events and sends these events through the cilium monitor1_2 socket. These events can be categorized as Event Sample or Lost Record. Event samples can be broken down into different categories: Agent events or Perf Events.
Access Log events are events such as DNS resolutions matching a cilium node policy while Agent Events can be any cilium agent events.
Perf Events are bpf related events such as drop, trace, policy verdict, or capture events.

The cilium plugin will listen on this socket for these events, decode the payload and reconstruct either an Agent Event or a Perf Event. These events are then decoded using a lightweight cilium parser. Once these events are decoded into a flow object, it is then passed to the external channel. The retina daemon listens for these events and send it to our monitor agent. Our hubble observer will consume these events and process the flows using our own custom [parsers](https://github.com/microsoft/retina/tree/main/pkg/hubble/parser).

### Code locations

- Plugin and eBPF code: *pkg/plugin/ciliumeventobserver/*


## Metrics

The metrics will be dependent on our custom parsers. For now, we have L34 parser and L7 parser for dns and http.
We currently do not support Agent or Access Log events from cilium itself.
This [metrics reference](https://docs.cilium.io/en/stable/observability/metrics/#metrics-reference) from cilium can give an idea of what metrics can be added.

At the moment, we can see metrics such as:
|        Name             | Description              | Extra Labels  |
| ----------------------- | -----------------------  | ------------- |
| `hubble_drop_total` | Number of drops | destination, protocol, reason, source |
| `hubble_tcp_flags_total` | TCP flag occurrences | destination, family, flag, source |
| `hubble_metrics_http_handler_request_duration_seconds` | A histogram of latencies of Hubble metrics handler. | code, le |
| `hubble_flows_processed_total` | Total number of flows processed | destination, protocol, subtype, type, verdict |
| `hubble_metrics_http_handler_requests_total` | A counter for requests to Hubble metrics handler. | code |
