use std::path::PathBuf;

fn main() {
    let ebpf_path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../ebpf/target/bpfel-unknown-none/release/dropreason-ebpf");

    // Create an empty stub so `include_bytes!` succeeds even before the eBPF
    // program has been compiled. The real binary is produced by
    // `cargo xtask build-ebpf --release`.
    if !ebpf_path.exists() {
        if let Some(parent) = ebpf_path.parent() {
            std::fs::create_dir_all(parent).expect("failed to create eBPF target directory");
        }
        std::fs::write(&ebpf_path, []).expect("failed to write eBPF stub");
        println!(
            "cargo::warning=dropreason eBPF binary not found; wrote empty stub. \
             Run `cargo xtask build-ebpf --release` for a real build."
        );
    }

    // Also create stub for the ringbuf variant.
    let ringbuf_path = ebpf_path.with_file_name("dropreason-ebpf-ringbuf");
    if !ringbuf_path.exists() {
        std::fs::write(&ringbuf_path, []).expect("failed to write ringbuf eBPF stub");
    }

    // Re-run if either eBPF binary changes (e.g. after a real build).
    println!("cargo::rerun-if-changed={}", ebpf_path.display());
    println!("cargo::rerun-if-changed={}", ringbuf_path.display());
}
