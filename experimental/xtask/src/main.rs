use std::path::PathBuf;
use std::process::Command;

use anyhow::{Context as _, bail};
use clap::Parser;

/// Workspace root directory (parent of xtask/).
fn workspace_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .unwrap()
        .to_path_buf()
}

fn run(cmd: &mut Command) -> anyhow::Result<()> {
    let status = cmd.status().context("failed to execute command")?;
    if !status.success() {
        bail!("command exited with status: {}", status);
    }
    Ok(())
}

/// Return the output of a command as a trimmed string.
fn output(cmd: &mut Command) -> anyhow::Result<String> {
    let out = cmd.output().context("failed to execute command")?;
    if !out.status.success() {
        bail!(
            "command failed: {}",
            String::from_utf8_lossy(&out.stderr).trim()
        );
    }
    Ok(String::from_utf8_lossy(&out.stdout).trim().to_string())
}

/// Detect whether the current kubectl context points to a kind cluster.
fn is_kind_cluster() -> bool {
    if let Ok(ctx) = output(Command::new("kubectl").args(["config", "current-context"])) {
        ctx.starts_with("kind-")
    } else {
        false
    }
}

#[derive(Parser)]
#[command(name = "xtask", about = "Retina experimental workspace tasks")]
enum Cli {
    /// Build the eBPF programs
    BuildEbpf {
        /// Build in release mode
        #[clap(long)]
        release: bool,
    },
    /// Build retina-agent binary
    BuildAgent {
        /// Build in release mode
        #[clap(long, default_value_t = true)]
        release: bool,
    },
    /// Build retina-operator binary
    BuildOperator {
        /// Build in release mode
        #[clap(long, default_value_t = true)]
        release: bool,
    },
    /// Build both agent and operator binaries
    Build,
    /// Build retina-agent container image
    ImageAgent {
        /// Docker image tag
        #[clap(long, default_value = "retina-rust:local")]
        tag: String,
    },
    /// Build retina-operator container image
    ImageOperator {
        /// Docker image tag
        #[clap(long, default_value = "retina-operator:local")]
        tag: String,
    },
    /// Build all container images
    Image,
    /// Build and deploy agent
    DeployAgent {
        /// Kind cluster name (only used for kind clusters)
        #[clap(long, default_value = "retina-test")]
        cluster: String,
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
        /// Container registry to push images to (e.g. "acnpublic.azurecr.io").
        /// If omitted, auto-detects: kind clusters use `kind load`, others require this flag.
        #[clap(long)]
        registry: Option<String>,
        /// Image tag suffix (default: "local")
        #[clap(long, default_value = "local")]
        tag: String,
    },
    /// Build and deploy operator
    DeployOperator {
        /// Kind cluster name (only used for kind clusters)
        #[clap(long, default_value = "retina-test")]
        cluster: String,
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
        /// Container registry to push images to
        #[clap(long)]
        registry: Option<String>,
        /// Image tag suffix (default: "local")
        #[clap(long, default_value = "local")]
        tag: String,
    },
    /// Deploy Hubble components (relay + UI)
    DeployHubble {
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
    },
    /// Build and deploy everything (operator + agent + hubble)
    Deploy {
        /// Kind cluster name (only used for kind clusters)
        #[clap(long, default_value = "retina-test")]
        cluster: String,
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
        /// Container registry to push images to (e.g. "acnpublic.azurecr.io").
        /// If omitted, auto-detects: kind clusters use `kind load`, others require this flag.
        #[clap(long)]
        registry: Option<String>,
        /// Image tag suffix (default: "local")
        #[clap(long, default_value = "local")]
        tag: String,
    },
    /// Set up kubectl port-forward to agent gRPC (4244)
    PortForward {
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
    },
    /// Kill stale retina-agent processes and port-forwards on port 4244
    KillStale,
    /// Kill everything on port 4244
    CleanPort,
    /// Tail agent logs
    Logs {
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
    },
}

fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();
    match cli {
        Cli::BuildEbpf { release } => build_ebpf(release),
        Cli::BuildAgent { release } => build_agent(release),
        Cli::BuildOperator { release } => build_operator(release),
        Cli::Build => {
            build_agent(true)?;
            build_operator(true)
        }
        Cli::ImageAgent { tag } => image_agent(&tag),
        Cli::ImageOperator { tag } => image_operator(&tag),
        Cli::Image => {
            image_agent("retina-rust:local")?;
            image_operator("retina-operator:local")
        }
        Cli::DeployAgent {
            cluster,
            namespace,
            registry,
            tag,
        } => deploy_agent(&cluster, &namespace, registry.as_deref(), &tag),
        Cli::DeployOperator {
            cluster,
            namespace,
            registry,
            tag,
        } => deploy_operator(&cluster, &namespace, registry.as_deref(), &tag),
        Cli::DeployHubble { namespace } => deploy_hubble(&namespace),
        Cli::Deploy {
            cluster,
            namespace,
            registry,
            tag,
        } => deploy_all(&cluster, &namespace, registry.as_deref(), &tag),
        Cli::PortForward { namespace } => port_forward(&namespace),
        Cli::KillStale => kill_stale(),
        Cli::CleanPort => clean_port(),
        Cli::Logs { namespace } => logs(&namespace),
    }
}

// ---------- Build ----------

fn build_ebpf(release: bool) -> anyhow::Result<()> {
    build_ebpf_plugin(release, "packetparser")?;
    build_ebpf_plugin(release, "dropreason")?;
    println!("All eBPF programs built successfully");
    Ok(())
}

/// Build a single eBPF plugin crate with both perf and ringbuf variants.
fn build_ebpf_plugin(release: bool, name: &str) -> anyhow::Result<()> {
    let manifest_path = workspace_dir()
        .join(format!("plugins/{name}/ebpf"))
        .join("Cargo.toml");
    if !manifest_path.exists() {
        bail!("cannot find plugins/{name}/ebpf/Cargo.toml");
    }

    let profile = if release { "release" } else { "debug" };
    let output_path = workspace_dir()
        .join(format!("plugins/{name}/ebpf/target/bpfel-unknown-none"))
        .join(profile)
        .join(format!("{name}-ebpf"));
    let ringbuf_path = output_path.with_file_name(format!("{name}-ebpf-ringbuf"));

    // 1. Build perf variant (default, no features).
    let mut cmd = Command::new("cargo");
    cmd.env_remove("RUSTUP_TOOLCHAIN")
        .args(["+nightly-2025-12-01", "build", "--manifest-path"])
        .arg(&manifest_path)
        .args(["--target=bpfel-unknown-none", "-Z", "build-std=core"]);
    if release {
        cmd.arg("--release");
    }
    run(&mut cmd)?;
    println!("{name} eBPF perf variant built");

    // Save the perf variant before the ringbuf build overwrites it.
    let perf_backup = output_path.with_file_name(format!("{name}-ebpf-perf-tmp"));
    std::fs::copy(&output_path, &perf_backup)
        .with_context(|| format!("failed to backup {name} perf eBPF binary"))?;

    // 2. Build ringbuf variant (--features ringbuf).
    let mut cmd = Command::new("cargo");
    cmd.env_remove("RUSTUP_TOOLCHAIN")
        .args(["+nightly-2025-12-01", "build", "--manifest-path"])
        .arg(&manifest_path)
        .args([
            "--target=bpfel-unknown-none",
            "-Z",
            "build-std=core",
            "--features",
            "ringbuf",
        ]);
    if release {
        cmd.arg("--release");
    }
    run(&mut cmd)?;
    println!("{name} eBPF ringbuf variant built");

    // 3. Move ringbuf output to its final name, restore perf variant.
    std::fs::rename(&output_path, &ringbuf_path)
        .with_context(|| format!("failed to rename {name} ringbuf eBPF binary"))?;
    std::fs::rename(&perf_backup, &output_path)
        .with_context(|| format!("failed to restore {name} perf eBPF binary"))?;

    println!("{name} eBPF programs built (perf + ringbuf)");
    Ok(())
}

fn build_agent(release: bool) -> anyhow::Result<()> {
    let mut cmd = Command::new("cargo");
    cmd.current_dir(workspace_dir())
        .args(["build", "--package", "retina-agent"]);
    if release {
        cmd.arg("--release");
    }
    run(&mut cmd)?;
    println!("retina-agent built successfully");
    Ok(())
}

fn build_operator(release: bool) -> anyhow::Result<()> {
    let mut cmd = Command::new("cargo");
    cmd.current_dir(workspace_dir())
        .args(["build", "--package", "retina-operator"]);
    if release {
        cmd.arg("--release");
    }
    run(&mut cmd)?;
    println!("retina-operator built successfully");
    Ok(())
}

// ---------- Container images ----------

fn image_agent(tag: &str) -> anyhow::Result<()> {
    let dir = workspace_dir();
    run(Command::new("docker").current_dir(&dir).args([
        "build",
        "-t",
        tag,
        "-f",
        "Dockerfile.local",
        ".",
    ]))?;
    println!("Agent image built: {tag}");
    Ok(())
}

fn image_operator(tag: &str) -> anyhow::Result<()> {
    let dir = workspace_dir();
    run(Command::new("docker").current_dir(&dir).args([
        "build",
        "-t",
        tag,
        "-f",
        "Dockerfile.operator.local",
        ".",
    ]))?;
    println!("Operator image built: {tag}");
    Ok(())
}

// ---------- Image distribution ----------

/// Resolve the registry to use. If `--registry` was given, use it.
/// Otherwise, if on a kind cluster, return None (use kind load).
/// Otherwise, error out.
fn resolve_registry(registry: Option<&str>) -> anyhow::Result<Option<String>> {
    if let Some(r) = registry {
        return Ok(Some(r.trim_end_matches('/').to_string()));
    }
    if is_kind_cluster() {
        return Ok(None);
    }
    bail!(
        "Not a kind cluster and --registry was not provided.\n\
         Use --registry <registry> to push images (e.g. --registry acnpublic.azurecr.io)"
    );
}

/// Load or push an image depending on the target cluster.
/// Returns the full image reference that was deployed.
fn distribute_image(
    local_tag: &str,
    remote_name: &str,
    remote_tag: &str,
    registry: Option<&str>,
    kind_cluster: &str,
) -> anyhow::Result<String> {
    match registry {
        Some(reg) => {
            let full_ref = format!("{reg}/{remote_name}:{remote_tag}");
            println!("Tagging {local_tag} → {full_ref}");
            run(Command::new("docker").args(["tag", local_tag, &full_ref]))?;
            println!("Pushing {full_ref}");
            run(Command::new("docker").args(["push", &full_ref]))?;
            Ok(full_ref)
        }
        None => {
            println!("Loading {local_tag} into kind cluster {kind_cluster}");
            run(Command::new("kind").args([
                "load",
                "docker-image",
                local_tag,
                "--name",
                kind_cluster,
            ]))?;
            Ok(local_tag.to_string())
        }
    }
}

// ---------- Helm ----------

/// Path to the Helm chart directory.
fn chart_dir() -> PathBuf {
    workspace_dir().join("deploy")
}

/// Split an image reference at the last `:` into (repository, tag).
fn split_image_ref(image_ref: &str) -> (&str, &str) {
    match image_ref.rfind(':') {
        Some(i) => (&image_ref[..i], &image_ref[i + 1..]),
        None => (image_ref, "latest"),
    }
}

/// Run `helm upgrade --install` with the given `--set` overrides.
fn helm_upgrade(namespace: &str, sets: &[(&str, &str)]) -> anyhow::Result<()> {
    let chart = chart_dir();
    let mut cmd = Command::new("helm");
    cmd.args(["upgrade", "--install", "retina-rust"]);
    cmd.arg(chart.as_os_str());
    cmd.args([
        "-n",
        namespace,
        "--create-namespace",
        "--reuse-values",
        "--wait",
        "--timeout",
        "120s",
    ]);
    for (k, v) in sets {
        cmd.args(["--set", &format!("{k}={v}")]);
    }
    run(&mut cmd)
}

// ---------- Deploy ----------

fn deploy_agent(
    cluster: &str,
    namespace: &str,
    registry: Option<&str>,
    tag: &str,
) -> anyhow::Result<()> {
    let registry = resolve_registry(registry)?;

    kill_stale()?;
    build_agent(true)?;

    let local_tag = "retina-rust:local";
    image_agent(local_tag)?;

    let image_ref = distribute_image(local_tag, "retina-rust", tag, registry.as_deref(), cluster)?;

    let (repo, img_tag) = split_image_ref(&image_ref);
    helm_upgrade(
        namespace,
        &[
            ("agent.image.repository", repo),
            ("agent.image.tag", img_tag),
        ],
    )?;

    if registry.is_none() {
        port_forward(namespace)?;
    }
    Ok(())
}

fn deploy_operator(
    cluster: &str,
    namespace: &str,
    registry: Option<&str>,
    tag: &str,
) -> anyhow::Result<()> {
    let registry = resolve_registry(registry)?;

    build_operator(true)?;

    let local_tag = "retina-operator:local";
    image_operator(local_tag)?;

    let image_ref = distribute_image(
        local_tag,
        "retina-operator",
        tag,
        registry.as_deref(),
        cluster,
    )?;

    let (repo, img_tag) = split_image_ref(&image_ref);
    helm_upgrade(
        namespace,
        &[
            ("operator.image.repository", repo),
            ("operator.image.tag", img_tag),
        ],
    )
}

fn deploy_hubble(namespace: &str) -> anyhow::Result<()> {
    println!("Deploying Hubble components (relay + UI)...");
    helm_upgrade(namespace, &[("hubble.enabled", "true")])?;
    println!("Hubble components deployed");
    Ok(())
}

fn deploy_all(
    cluster: &str,
    namespace: &str,
    registry: Option<&str>,
    tag: &str,
) -> anyhow::Result<()> {
    let registry = resolve_registry(registry)?;

    kill_stale()?;

    // Build both binaries and images.
    build_agent(true)?;
    build_operator(true)?;

    let agent_local = "retina-rust:local";
    let operator_local = "retina-operator:local";
    image_agent(agent_local)?;
    image_operator(operator_local)?;

    let agent_ref = distribute_image(
        agent_local,
        "retina-rust",
        tag,
        registry.as_deref(),
        cluster,
    )?;
    let operator_ref = distribute_image(
        operator_local,
        "retina-operator",
        tag,
        registry.as_deref(),
        cluster,
    )?;

    let (agent_repo, agent_tag) = split_image_ref(&agent_ref);
    let (operator_repo, operator_tag) = split_image_ref(&operator_ref);

    helm_upgrade(
        namespace,
        &[
            ("agent.image.repository", agent_repo),
            ("agent.image.tag", agent_tag),
            ("operator.image.repository", operator_repo),
            ("operator.image.tag", operator_tag),
            ("hubble.enabled", "true"),
        ],
    )?;

    if registry.is_none() {
        port_forward(namespace)?;
    }
    Ok(())
}

// ---------- Dev helpers ----------

fn port_forward(namespace: &str) -> anyhow::Result<()> {
    // Kill existing port-forwards first.
    let _ = Command::new("pkill")
        .args(["-f", "kubectl.*port-forward.*4244"])
        .status();
    std::thread::sleep(std::time::Duration::from_secs(1));

    Command::new("kubectl")
        .args([
            "port-forward",
            "-n",
            namespace,
            "daemonset/retina-agent",
            "4244:4244",
        ])
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .spawn()
        .context("failed to start port-forward")?;

    println!("Port-forward active: localhost:4244 → retina-agent");
    Ok(())
}

fn kill_stale() -> anyhow::Result<()> {
    println!("Checking for stale processes on gRPC port 4244...");

    // Kill stale retina-agent processes.
    if let Ok(output) = Command::new("sudo").args(["lsof", "-ti", ":4244"]).output() {
        let pids = String::from_utf8_lossy(&output.stdout);
        for pid_str in pids.split_whitespace() {
            if let Ok(pid) = pid_str.parse::<u32>() {
                // Check if it's a retina-agent process.
                if let Ok(comm) = Command::new("ps")
                    .args(["-p", &pid.to_string(), "-o", "comm="])
                    .output()
                {
                    let name = String::from_utf8_lossy(&comm.stdout).trim().to_string();
                    if name == "retina-agent" || name == "retina-ag" {
                        println!("  Killing stale retina-agent (PID {pid})");
                        let _ = Command::new("sudo")
                            .args(["kill", &pid.to_string()])
                            .status();
                    }
                }
            }
        }
    }

    // Kill stale kubectl port-forwards.
    let _ = Command::new("pkill")
        .args(["-f", "kubectl.*port-forward.*4244"])
        .status();

    Ok(())
}

fn clean_port() -> anyhow::Result<()> {
    if let Ok(output) = Command::new("sudo").args(["lsof", "-ti", ":4244"]).output() {
        let pids = String::from_utf8_lossy(&output.stdout);
        for pid_str in pids.split_whitespace() {
            let _ = Command::new("sudo").args(["kill", pid_str.trim()]).status();
        }
    }
    println!("Port 4244 cleared");
    Ok(())
}

fn logs(namespace: &str) -> anyhow::Result<()> {
    run(Command::new("kubectl").args(["logs", "-n", namespace, "daemonset/retina-agent", "-f"]))
}
