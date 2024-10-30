# `linuxutil`

Gathers TCP/UDP statistics and network interface statistics from the `netstats` and `ethtool` Node utilities (respectively).

## Capabilities

The `linuxutil` plugin requires the `CAP_BPF` capability.

## Architecture

The plugin uses the following utilities as data sources:

1. `netstat`
    - TCP Socket information
    - UDP Socket information
    - "/proc/net/netstat" for IP and UDP statistics
2. `ethtool`
    - Interface statistics

### Code Locations

- Plugin code interfacing with the Node utilities: *pkg/plugin/linuxutil/*

## Metrics

See metrics for [Basic Mode](../../modes/basic.md#plugin-linuxutil-linux) (Advanced modes have identical metrics).

### Configuration (in Code)

Both `ethtool` and `netstat` data can be curated to remove unwanted data. Below options in a struct in *linuxutil.go* can be used to configure the same.

```go
type EthtoolOpts struct {
 // when true will only include keys with err or drop in its name
 errOrDropKeysOnly bool

 // when true will include all keys with value 0
 addZeroVal bool
}
```

```go
type NetstatOpts struct {
 // when true only includes curated list of keys
 CuratedKeys bool

 // when true will include all keys with value 0
 AddZeroVal bool

 // get only listening sockets
 ListenSock bool
}
```

These are initialized in the linuxutil.go file.

## Label Values for `tcp_connection_stats`

Below is a running list of all statistics for the metric `tcp_connection_stats`, captured from the `netstats` utility:

- `DelayedACKLocked`
- `DelayedACKLost`
- `DelayedACKs`
- `IPReversePathFilter`
- `PAWSEstab`
- `TCPACKSkippedPAWS`
- `TCPACKSkippedSeq`
- `TCPAbortOnClose`
- `TCPAbortOnData`
- `TCPAckCompressed`
- `TCPAutoCorking`
- `TCPBacklogCoalesce`
- `TCPChallengeACK`
- `TCPDSACKIgnoredNoUndo`
- `TCPDSACKOldSent`
- `TCPDSACKRecv`
- `TCPDSACKRecvSegs`
- `TCPDSACKUndo`
- `TCPDelivered`
- `TCPFastRetrans`
- `TCPFromZeroWindowAdv`
- `TCPFullUndo`
- `TCPHPAcks`
- `TCPHPHits`
- `TCPHystartTrainCwnd`
- `TCPHystartTrainDetect`
- `TCPKeepAlive`
- `TCPLossProbeRecovery`
- `TCPLossProbes`
- `TCPLossUndo`
- `TCPOFOQueue`
- `TCPOrigDataSent`
- `TCPPartialUndo`
- `TCPPureAcks`
- `TCPRcvCoalesce`
- `TCPSACKReorder`
- `TCPSackMerged`
- `TCPSackRecovery`
- `TCPSackShiftFallback`
- `TCPSackShifted`
- `TCPSpuriousRtxHostQueues`
- `TCPSynRetrans`
- `TCPTSReorder`
- `TCPTimeouts`
- `TCPToZeroWindowAdv`
- `TCPWantZeroWindowAdv`
- `TW`
- `TWRecycled`
- `TcpDuplicateDataRehash`
- `TcpTimeoutRehash`
