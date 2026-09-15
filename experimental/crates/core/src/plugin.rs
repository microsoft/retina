//! Plugin trait and context for eBPF-based data collection plugins.

use std::sync::Arc;

use tokio::sync::broadcast;

use crate::ipcache::IpCache;
use crate::metrics::{AgentState, Metrics};
use crate::store::FlowStore;

/// Shared infrastructure passed to every plugin at startup.
pub struct PluginContext {
    pub flow_tx: broadcast::Sender<Arc<retina_proto::flow::Flow>>,
    pub flow_store: Arc<FlowStore>,
    pub ip_cache: Arc<IpCache>,
    pub metrics: Arc<Metrics>,
    pub state: Arc<AgentState>,
}

/// Lifecycle interface for a Retina data-plane plugin.
///
/// Implementors own their own tasks (spawned in `start`). `stop` is
/// responsible for aborting those tasks and releasing resources.
#[async_trait::async_trait]
pub trait Plugin: Send + Sync {
    /// Short stable identifier, e.g. `"packetparser"`.
    fn name(&self) -> &str;

    /// Start the plugin. The plugin takes ownership of `ctx` and may
    /// clone `Arc`s from it for use in spawned tasks.
    async fn start(&mut self, ctx: PluginContext) -> anyhow::Result<()>;

    /// Gracefully stop all plugin tasks.
    async fn stop(&mut self) -> anyhow::Result<()>;
}
