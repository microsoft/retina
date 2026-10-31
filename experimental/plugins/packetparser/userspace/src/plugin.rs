use std::sync::{Arc, Mutex};

use anyhow::Context as _;
use retina_core::plugin::{Plugin, PluginContext};
use tokio::task::JoinHandle;
use tracing::{error, info, warn};

use crate::loader;

/// `PacketParser` plugin: loads eBPF TC classifiers, reads events (ring buffer
/// or perf), converts to Hubble flows, and optionally watches for new veth
/// interfaces.
pub struct PacketParser {
    extra_interfaces: Vec<String>,
    pod_level: bool,
    sampling_rate: u32,
    ring_buffer_size: u32,
    /// Event reader runs on dedicated OS thread(s), not the tokio runtime.
    event_handle: Option<std::thread::JoinHandle<()>>,
    gc_handle: Option<JoinHandle<()>>,
    watcher_handle: Option<JoinHandle<()>>,
    log_handle: Option<JoinHandle<()>>,
    ebpf: Option<Arc<Mutex<aya::Ebpf>>>,
}

impl PacketParser {
    pub fn new(
        extra_interfaces: Vec<String>,
        pod_level: bool,
        sampling_rate: u32,
        ring_buffer_size: u32,
    ) -> Self {
        Self {
            extra_interfaces,
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
    fn name(&self) -> &'static str {
        "packetparser"
    }

    async fn start(&mut self, ctx: PluginContext) -> anyhow::Result<()> {
        let (mut ebpf, event_source, conntrack_map) = loader::load_and_attach(
            &self.extra_interfaces,
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
        self.ebpf = Some(Arc::clone(&ebpf));

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

        // Spawn event reader on dedicated OS thread(s) to avoid contending
        // with async tasks on the tokio runtime.
        let flow_tx = ctx.flow_tx.clone();
        let flow_store = Arc::clone(&ctx.flow_store);
        let ip_cache = Arc::clone(&ctx.ip_cache);
        let metrics = Arc::clone(&ctx.metrics);
        let state = Arc::clone(&ctx.state);
        self.event_handle = Some(match event_source {
            loader::EventSource::Ring(ring_buf) => {
                info!("using BPF ring buffer for event delivery");
                std::thread::Builder::new()
                    .name("retina-ring".into())
                    .spawn(move || {
                        crate::events::run_ring_reader(
                            ring_buf, flow_tx, flow_store, ip_cache, metrics, state,
                        );
                    })
                    .context("failed to spawn ring buffer reader thread")?
            }
            loader::EventSource::Perf(perf_array) => {
                info!("using perf event array for event delivery");
                std::thread::Builder::new()
                    .name("retina-perf".into())
                    .spawn(move || {
                        if let Err(e) = crate::events::run_perf_reader(
                            perf_array, flow_tx, flow_store, ip_cache, metrics, state,
                        ) {
                            error!("perf reader error: {}", e);
                        }
                    })
                    .context("failed to spawn perf reader thread")?
            }
        });

        // Spawn conntrack GC.
        let gc_metrics = Arc::clone(&ctx.metrics);
        self.gc_handle = Some(tokio::spawn(async move {
            crate::conntrack_gc::run_gc(conntrack_map, gc_metrics).await;
        }));

        // Conditionally spawn veth watcher for pod-level monitoring.
        if self.pod_level {
            let ebpf_clone = Arc::clone(&ebpf);
            let watcher_ip_cache = Arc::clone(&ctx.ip_cache);
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
        // Drop the event reader thread handle (detaches; thread exits on process shutdown).
        drop(self.event_handle.take());

        for handle in [
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
