//! Packetparser plugin: eBPF TC classifier loading, event reading,
//! conntrack GC, and endpoint veth watching.

pub mod conntrack_gc;
pub mod events;
pub mod loader;
pub mod plugin;
pub mod watcher;
