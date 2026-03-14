//! Dropreason plugin: eBPF fexit/tracepoint loading, drop event reading,
//! and per-CPU metrics aggregation.

pub mod events;
pub mod loader;
pub mod plugin;
