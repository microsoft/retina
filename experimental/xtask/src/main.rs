use std::path::PathBuf;
use std::process::Command;

use anyhow::{bail, Context as _};
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
    /// Build and deploy agent to kind cluster
    DeployAgent {
        /// Kind cluster name
        #[clap(long, default_value = "retina-test")]
        cluster: String,
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
    },
    /// Build and deploy operator to kind cluster
    DeployOperator {
        /// Kind cluster name
        #[clap(long, default_value = "retina-test")]
        cluster: String,
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
    },
    /// Build and deploy everything to kind cluster
    Deploy {
        /// Kind cluster name
        #[clap(long, default_value = "retina-test")]
        cluster: String,
        /// Kubernetes namespace
        #[clap(long, default_value = "retina-rust")]
        namespace: String,
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
        Cli::DeployAgent { cluster, namespace } => deploy_agent(&cluster, &namespace),
        Cli::DeployOperator { cluster, namespace } => deploy_operator(&cluster, &namespace),
        Cli::Deploy { cluster, namespace } => {
            deploy_operator(&cluster, &namespace)?;
            deploy_agent(&cluster, &namespace)
        }
        Cli::PortForward { namespace } => port_forward(&namespace),
        Cli::KillStale => kill_stale(),
        Cli::CleanPort => clean_port(),
        Cli::Logs { namespace } => logs(&namespace),
    }
}

// ---------- Build ----------

fn build_ebpf(release: bool) -> anyhow::Result<()> {
    let manifest_path = workspace_dir()
        .join("plugins/packetparser/ebpf")
        .join("Cargo.toml");
    if !manifest_path.exists() {
        bail!("cannot find plugins/packetparser/ebpf/Cargo.toml");
    }

    let mut cmd = Command::new("cargo");
    cmd.env_remove("RUSTUP_TOOLCHAIN")
        .args(["+nightly-2025-12-01", "build", "--manifest-path"])
        .arg(&manifest_path)
        .args(["--target=bpfel-unknown-none", "-Z", "build-std=core"]);
    if release {
        cmd.arg("--release");
    }

    run(&mut cmd)?;
    println!("eBPF programs built successfully");
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
    run(
        Command::new("docker")
            .current_dir(&dir)
            .args(["build", "-t", tag, "-f", "Dockerfile.local", "."]),
    )?;
    println!("Agent image built: {tag}");
    Ok(())
}

fn image_operator(tag: &str) -> anyhow::Result<()> {
    let dir = workspace_dir();
    run(
        Command::new("docker")
            .current_dir(&dir)
            .args(["build", "-t", tag, "-f", "Dockerfile.operator.local", "."]),
    )?;
    println!("Operator image built: {tag}");
    Ok(())
}

// ---------- Deploy to kind ----------

fn deploy_agent(cluster: &str, namespace: &str) -> anyhow::Result<()> {
    let dir = workspace_dir();
    let tag = "retina-rust:local";

    kill_stale()?;
    build_agent(true)?;
    image_agent(tag)?;

    run(
        Command::new("kind")
            .args(["load", "docker-image", tag, "--name", cluster]),
    )?;
    run(
        Command::new("kubectl")
            .current_dir(&dir)
            .args(["apply", "-f", "deploy.yaml"]),
    )?;
    run(Command::new("kubectl").args([
        "rollout", "restart",
        &format!("daemonset/retina-agent"),
        "-n", namespace,
    ]))?;
    run(Command::new("kubectl").args([
        "rollout", "status",
        "daemonset/retina-agent",
        "-n", namespace,
        "--timeout=60s",
    ]))?;

    port_forward(namespace)
}

fn deploy_operator(cluster: &str, namespace: &str) -> anyhow::Result<()> {
    let dir = workspace_dir();
    let tag = "retina-operator:local";

    build_operator(true)?;
    image_operator(tag)?;

    run(
        Command::new("kind")
            .args(["load", "docker-image", tag, "--name", cluster]),
    )?;
    run(
        Command::new("kubectl")
            .current_dir(&dir)
            .args(["apply", "-f", "deploy-operator.yaml"]),
    )?;
    run(Command::new("kubectl").args([
        "rollout", "restart",
        "deployment/retina-operator",
        "-n", namespace,
    ]))?;
    run(Command::new("kubectl").args([
        "rollout", "status",
        "deployment/retina-operator",
        "-n", namespace,
        "--timeout=60s",
    ]))
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
            "-n", namespace,
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
    if let Ok(output) = Command::new("sudo")
        .args(["lsof", "-ti", ":4244"])
        .output()
    {
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
    if let Ok(output) = Command::new("sudo")
        .args(["lsof", "-ti", ":4244"])
        .output()
    {
        let pids = String::from_utf8_lossy(&output.stdout);
        for pid_str in pids.split_whitespace() {
            let _ = Command::new("sudo")
                .args(["kill", pid_str.trim()])
                .status();
        }
    }
    println!("Port 4244 cleared");
    Ok(())
}

fn logs(namespace: &str) -> anyhow::Result<()> {
    run(Command::new("kubectl").args([
        "logs",
        "-n", namespace,
        "daemonset/retina-agent",
        "-f",
    ]))
}
