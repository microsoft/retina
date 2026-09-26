use std::collections::HashSet;
use std::sync::Arc;

use anyhow::Context as _;
use retina_core::plugin::{Plugin, PluginContext};
use tokio::task::JoinHandle;
use tracing::{error, info, warn};

use crate::loader;

pub struct DropReasonPlugin {
    ring_buffer_size: u32,
    suppressed_reasons: HashSet<String>,
    event_handle: Option<std::thread::JoinHandle<()>>,
    metrics_handle: Option<JoinHandle<()>>,
    log_handle: Option<JoinHandle<()>>,
    _ebpf: Option<aya::Ebpf>,
}

impl DropReasonPlugin {
    pub fn new(ring_buffer_size: u32, suppressed_reasons: HashSet<String>) -> Self {
        Self {
            ring_buffer_size,
            suppressed_reasons,
            event_handle: None,
            metrics_handle: None,
            log_handle: None,
            _ebpf: None,
        }
    }
}

#[async_trait::async_trait]
impl Plugin for DropReasonPlugin {
    fn name(&self) -> &'static str {
        "dropreason"
    }

    async fn start(&mut self, ctx: PluginContext) -> anyhow::Result<()> {
        let loader::EbpfHandles {
            mut ebpf,
            event_source,
            metrics_map,
            kernel_drop_reasons,
            suppressed_reasons,
            ring_lost_map,
        } = loader::load_and_attach(self.ring_buffer_size, &self.suppressed_reasons)
            .context("failed to load dropreason eBPF programs")?;

        let kernel_drop_reasons = Arc::new(kernel_drop_reasons);
        let suppressed_reasons = Arc::new(suppressed_reasons);

        // Set up aya-log forwarding (best-effort).
        if let Ok(mut logger) = aya_log::EbpfLogger::init(&mut ebpf) {
            let handle = tokio::spawn(async move {
                use std::os::fd::AsFd as _;
                let owned_fd = match logger.as_fd().try_clone_to_owned() {
                    Ok(fd) => fd,
                    Err(e) => {
                        warn!("dropreason: clone logger fd: {e}");
                        return;
                    }
                };
                let async_fd = match tokio::io::unix::AsyncFd::with_interest(
                    owned_fd,
                    tokio::io::Interest::READABLE,
                ) {
                    Ok(afd) => afd,
                    Err(e) => {
                        warn!("dropreason: AsyncFd for logger: {e}");
                        return;
                    }
                };
                loop {
                    if let Ok(mut guard) = async_fd.readable().await {
                        logger.flush();
                        guard.clear_ready();
                    }
                }
            });
            self.log_handle = Some(handle);
        }

        // Keep the Ebpf handle alive so eBPF programs stay attached.
        self._ebpf = Some(ebpf);

        // Spawn event reader on a dedicated OS thread.
        let flow_tx = ctx.flow_tx.clone();
        let flow_store = Arc::clone(&ctx.flow_store);
        let ip_cache = Arc::clone(&ctx.ip_cache);
        let metrics = Arc::clone(&ctx.metrics);
        let state = Arc::clone(&ctx.state);
        let kdr = Arc::clone(&kernel_drop_reasons);
        let sr = Arc::clone(&suppressed_reasons);

        self.event_handle = Some(match event_source {
            loader::EventSource::Ring(ring_buf) => {
                info!("dropreason: using ring buffer for events");
                std::thread::Builder::new()
                    .name("retina-drop-ring".into())
                    .spawn(move || {
                        crate::events::run_ring_reader(
                            ring_buf, flow_tx, flow_store, ip_cache, metrics, state, kdr, sr,
                        );
                    })
                    .context("spawn dropreason ring reader")?
            }
            loader::EventSource::Perf(perf_array) => {
                info!("dropreason: using perf event array for events");
                std::thread::Builder::new()
                    .name("retina-drop-perf".into())
                    .spawn(move || {
                        if let Err(e) = crate::events::run_perf_reader(
                            perf_array, flow_tx, flow_store, ip_cache, metrics, state, kdr, sr,
                        ) {
                            error!("dropreason perf reader error: {e}");
                        }
                    })
                    .context("spawn dropreason perf reader")?
            }
        });

        // Spawn periodic metrics map reader.
        let metrics_clone = Arc::clone(&ctx.metrics);
        self.metrics_handle = Some(tokio::spawn(async move {
            crate::events::run_metrics_reader(
                metrics_map,
                metrics_clone,
                kernel_drop_reasons,
                ring_lost_map,
            )
            .await;
        }));

        info!("dropreason plugin started");
        Ok(())
    }

    async fn stop(&mut self) -> anyhow::Result<()> {
        // The event reader thread will exit when the process shuts down.
        drop(self.event_handle.take());

        for handle in [self.metrics_handle.take(), self.log_handle.take()]
            .into_iter()
            .flatten()
        {
            handle.abort();
        }

        // Drop the Ebpf object — fexit programs are auto-detached.
        self._ebpf = None;
        info!("dropreason plugin stopped");
        Ok(())
    }
}
