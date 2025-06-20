# Debug Overview

The Retina debug functionality provides real-time network monitoring and troubleshooting capabilities. Unlike captures which record network traffic for later analysis, debug commands offer live insights into network behavior and issues.

## Available Debug Commands

### Drop Event Monitoring

The `kubectl retina debug drop` command monitors packet drop events in real-time using eBPF technology. This helps network operators and developers quickly identify and troubleshoot packet loss issues.

**Key Features:**

- Real-time monitoring of packet drops
- Detailed drop reason information
- Source and destination analysis
- Customizable filtering by IP addresses
- Console and file output options
- Word-wrapped display for various terminal widths

**Use Cases:**

- Troubleshooting connectivity issues
- Monitoring network security events
- Performance analysis and optimization
- Network debugging during development

## Architecture

The debug commands leverage Retina's existing eBPF plugins to provide real-time monitoring capabilities:

```text
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   kubectl CLI   │────▶│  Debug Command  │────▶│   eBPF Plugin   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                       │
                                ▼                       ▼
                        ┌─────────────────┐    ┌─────────────────┐
                        │  Console/File   │    │  Kernel Events  │
                        │     Output      │    │   (Live Data)   │
                        └─────────────────┘    └─────────────────┘
```

## Requirements

- **Linux Environment**: eBPF support requires Linux
- **Kernel Version**: Modern Linux kernel (4.9+)
- **Privileges**: May require elevated privileges for eBPF operations
- **Memory Limits**: Sufficient memory lock limits for eBPF maps

## Getting Started

1. [Install the Retina CLI](../02-Installation/02-CLI.md)
2. Review the [CLI debug documentation](01-cli.md)
3. Start with basic monitoring: `kubectl retina debug drop --duration=30s`

## Comparison with Captures

| Feature | Debug Commands | Capture Commands |
|---------|----------------|------------------|
| **Timing** | Real-time | Record and analyze |
| **Storage** | Optional file output | Always stored |
| **Duration** | Live monitoring | Fixed time windows |
| **Use Case** | Active troubleshooting | Forensic analysis |
| **Resource Usage** | Low (streaming) | Higher (storage) |
| **Analysis** | Immediate feedback | Post-capture analysis |

## Future Enhancements

The debug functionality is designed to be extensible. Future debug commands may include:

- Connection tracking monitoring
- DNS resolution debugging
- Performance metrics monitoring
- Security event analysis
