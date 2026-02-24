use std::process::Command;

fn main() {
    // Git commit SHA (short).
    let sha = run("git", &["rev-parse", "--short", "HEAD"]);
    println!("cargo:rustc-env=GIT_COMMIT={sha}");

    // Git tag if HEAD is exactly on a tag, otherwise "dev".
    let tag = run("git", &["describe", "--tags", "--exact-match"]);
    let version = if tag.is_empty() { "dev".to_string() } else { tag };
    println!("cargo:rustc-env=GIT_VERSION={version}");

    // Rust compiler version.
    let rustc = run("rustc", &["--version"]);
    println!("cargo:rustc-env=RUSTC_VERSION={rustc}");

    // Rebuild if git HEAD changes (new commit, tag, or checkout).
    println!("cargo:rerun-if-changed=../../.git/HEAD");
    println!("cargo:rerun-if-changed=../../.git/refs/");
}

fn run(cmd: &str, args: &[&str]) -> String {
    Command::new(cmd)
        .args(args)
        .output()
        .ok()
        .filter(|o| o.status.success())
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .unwrap_or_default()
}
