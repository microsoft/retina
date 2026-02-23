use std::future::Future;
use std::time::Duration;
use tracing::{error, info, warn};

const INITIAL_BACKOFF: Duration = Duration::from_secs(1);
const MAX_BACKOFF: Duration = Duration::from_secs(60);

/// Run an async operation in a loop with exponential backoff on failure.
///
/// - On `Ok(())` (clean stream end): resets backoff and restarts immediately after a 1s pause.
/// - On transient error (connection lost, h2 reset): logs at WARN, resets backoff, reconnects.
/// - On other errors: logs at ERROR, sleeps with exponential backoff (1s → 60s max), then retries.
/// - Never returns — loops forever.
pub async fn retry_with_backoff<F, Fut>(label: &str, mut f: F) -> !
where
    F: FnMut() -> Fut,
    Fut: Future<Output = anyhow::Result<()>>,
{
    let mut backoff = INITIAL_BACKOFF;

    loop {
        info!("starting {label}");
        match f().await {
            Ok(()) => {
                warn!("{label} stream ended, reconnecting");
                backoff = INITIAL_BACKOFF;
            }
            Err(e) => {
                if is_transient(&e) {
                    warn!("{label} connection lost, reconnecting: {e}");
                    backoff = INITIAL_BACKOFF;
                } else {
                    error!(backoff_secs = backoff.as_secs(), "{label} error: {e}");
                    backoff = (backoff * 2).min(MAX_BACKOFF);
                }
            }
        }
        tokio::time::sleep(backoff).await;
    }
}

/// Returns `true` for transient connection/transport errors that are expected
/// during rolling updates, operator restarts, and network blips.
fn is_transient(err: &anyhow::Error) -> bool {
    let msg = format!("{err:#}");
    msg.contains("h2 protocol error")
        || msg.contains("connection reset")
        || msg.contains("broken pipe")
        || msg.contains("connection refused")
        || msg.contains("transport error")
        || msg.contains("dns error")
        || msg.contains("connection closed")
        || msg.contains("channel closed")
}
