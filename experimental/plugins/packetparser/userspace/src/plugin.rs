use std::sync::{Arc, Mutex};

use anyhow::Context as _;
use retina_core::plugin::{Plugin, PluginContext};
use tokio::task::JoinHandle;
use tracing::{error, info, warn};

use crate::loader;

/// PacketParser plugin: loads eBPF TC classifiers, reads events (ring buffer
/// or perf), converts to Hubble flows, and optionally watches for new veth
/// interfaces.
pub struct PacketParser {
    host_iface: Option<String>,
    pod_level: bool,
    sampling_rate: u32,
    ring_buffer_size: u32,
    event_handle: Option<JoinHandle<()>>,
    gc_handle: Option<JoinHandle<()>>,
    watcher_handle: Option<JoinHandle<()>>,
    log_handle: Option<JoinHandle<()>>,
    ebpf: Option<Arc<Mutex<aya::Ebpf>>>,
}

impl PacketParser {
    pub fn new(
        host_iface: Option<String>,
        pod_level: bool,
        sampling_rate: u32,
        ring_buffer_size: u32,
    ) -> Self {
        Self {
            host_iface,
            pod_level,
            sampling_rate,
            ring_buffer_size,
            event_handle: None,
            gc_handle: None,
            watcher_handle: None,
            log_handle: None,
            ebpf: None,
        }
    }
}

#[async_trait::async_trait]
impl Plugin for PacketParser {
    fn name(&self) -> &str {
        "packetparser"
    }

    async fn start(&mut self, ctx: PluginContext) -> anyhow::Result<()> {
        let (mut ebpf, event_source, conntrack_map) = loader::load_and_attach(
            self.host_iface.as_deref(),
            self.sampling_rate,
            self.ring_buffer_size,
        )
        .context("failed to load and attach eBPF programs")?;

        // Set up aya-log forwarding (best-effort).
        let ebpf_logger = match aya_log::EbpfLogger::init(&mut ebpf) {
            Ok(logger) => Some(logger),
            Err(e) => {
                warn!("failed to initialize eBPF logger: {}", e);
                None
            }
        };

        let ebpf = Arc::new(Mutex::new(ebpf));
        self.ebpf = Some(ebpf.clone());

        // Spawn eBPF log reader (best-effort: log and exit on setup failure).
        if let Some(logger) = ebpf_logger {
            let handle = tokio::spawn(async move {
                use std::os::fd::AsFd as _;
                let owned_fd = match logger.as_fd().try_clone_to_owned() {
                    Ok(fd) => fd,
                    Err(e) => {
                        warn!("failed to clone eBPF logger fd: {e}");
                        return;
                    }
                };
                let async_fd = match tokio::io::unix::AsyncFd::with_interest(
                    owned_fd,
                    tokio::io::Interest::READABLE,
                ) {
                    Ok(afd) => afd,
                    Err(e) => {
                        warn!("failed to create AsyncFd for eBPF logger: {e}");
                        return;
                    }
                };
                let mut logger = logger;
                loop {
                    if let Ok(mut guard) = async_fd.readable().await {
                        logger.flush();
                        guard.clear_ready();
                    }
                }
            });
            self.log_handle = Some(handle);
        }

        // Spawn event reader (ring buffer or perf, depending on kernel support).
        let flow_tx = ctx.flow_tx.clone();
        let flow_store = ctx.flow_store.clone();
        let ip_cache = ctx.ip_cache.clone();
        let metrics = ctx.metrics.clone();
        let state = ctx.state.clone();
        self.event_handle = Some(match event_source {
            loader::EventSource::Ring(ring_buf) => {
                info!("using BPF ring buffer for event delivery");
                tokio::spawn(async move {
                    if let Err(e) = crate::events::run_ring_reader(
                        ring_buf, flow_tx, flow_store, ip_cache, metrics, state,
                    )
                    .await
                    {
                        error!("ring buffer reader error: {}", e);
                    }
                })
            }
            loader::EventSource::Perf(perf_array) => {
                info!("using perf event array for event delivery");
                tokio::spawn(async move {
                    if let Err(e) = crate::events::run_perf_reader(
                        perf_array, flow_tx, flow_store, ip_cache, metrics, state,
                    )
                    .await
                    {
                        error!("perf reader error: {}", e);
                    }
                })
            }
        });

        // Spawn conntrack GC.
        let gc_metrics = ctx.metrics.clone();
        self.gc_handle = Some(tokio::spawn(async move {
            crate::conntrack_gc::run_gc(conntrack_map, gc_metrics).await;
        }));

        // Conditionally spawn veth watcher for pod-level monitoring.
        if self.pod_level {
            let ebpf_clone = ebpf.clone();
            let watcher_ip_cache = ctx.ip_cache.clone();
            self.watcher_handle = Some(tokio::spawn(async move {
                let mut w = crate::watcher::VethWatcher::new(ebpf_clone, watcher_ip_cache);
                if let Err(e) = w.run().await {
                    error!("veth watcher error: {e}");
                }
            }));
        }

        ctx.state
            .plugin_started
            .store(true, std::sync::atomic::Ordering::Release);
        info!("packetparser plugin started");
        Ok(())
    }

    async fn stop(&mut self) -> anyhow::Result<()> {
        for handle in [
            self.event_handle.take(),
            self.gc_handle.take(),
            self.watcher_handle.take(),
            self.log_handle.take(),
        ]
        .into_iter()
        .flatten()
        {
            handle.abort();
        }

        // Drop eBPF object — TC filters are auto-removed.
        self.ebpf = None;

        info!("packetparser plugin stopped");
        Ok(())
    }
}
