# pinned base images

# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/rust:1.90 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/rust@sha256:23e2b2fff469b69ee755170d75b46053dff255250550679d3342d92bf33e3d8f AS rust

# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core@sha256:9948138108a3d69f1dae62104599ac03132225c3b7a5ac57b85a214629c8567d AS azurelinux-core

# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/distroless/minimal@sha256:0801b80a0927309572b9adc99bd1813bc680473175f6e8175cd4124d95dbd50c AS azurelinux-distroless

# Stage 1: Build eBPF programs (needs nightly + bpf-linker)
FROM rust AS ebpf-builder
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain none
ENV PATH="/root/.cargo/bin:${PATH}"
RUN rustup toolchain install nightly-2025-12-01 --component rust-src
RUN cargo +nightly-2025-12-01 install bpf-linker
WORKDIR /build
COPY . .
RUN cargo xtask build-ebpf --release

# Stage 2: Build userspace agent (stable Rust + protoc)
FROM rust AS agent-builder
RUN tdnf install -y \
    protobuf-devel \
    pkgconf-pkg-config \
    elfutils-libelf-devel \
    zlib-devel \
    && tdnf clean all
WORKDIR /build
COPY . .
COPY --from=ebpf-builder /build/plugins/packetparser/ebpf/target/bpfel-unknown-none/release/packetparser-ebpf \
    ./plugins/packetparser/ebpf/target/bpfel-unknown-none/release/packetparser-ebpf
RUN cargo build --release --package retina-agent

# Tools: networking debug utilities
FROM azurelinux-core AS tools
RUN tdnf install -y iproute && tdnf clean all
RUN mkdir -p /tmp/bin && \
    cp $(command -v ip) $(command -v ss) $(command -v tc) /tmp/bin/

# Stage 3: Minimal runtime
FROM azurelinux-distroless
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /tmp/bin/ /bin/
COPY --from=agent-builder /build/target/release/retina-agent /usr/local/bin/
ENTRYPOINT ["retina-agent"]
