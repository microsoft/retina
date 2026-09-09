//! Shared eBPF loading utilities used by both packetparser and dropreason plugins.

use tracing::warn;

/// Force 8-byte alignment on embedded byte data so the `object` crate's ELF
/// parser can cast the pointer to `Elf64_Ehdr` without misalignment.
/// (`include_bytes!` only guarantees 1-byte alignment.)
#[repr(C, align(8))]
pub struct Align8<Bytes: ?Sized> {
    pub bytes: Bytes,
}

/// Check if the running kernel supports BPF ring buffers (>= 5.8).
#[must_use]
pub fn kernel_supports_ringbuf() -> bool {
    unsafe {
        let mut utsname: libc::utsname = core::mem::zeroed();
        if libc::uname(&raw mut utsname) != 0 {
            return false;
        }
        let release = core::ffi::CStr::from_ptr(utsname.release.as_ptr());
        let release = release.to_string_lossy();
        // Parse "major.minor..." from uname release string.
        let mut parts = release.split('.');
        let major: u32 = parts.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        let minor: u32 = parts.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        major > 5 || (major == 5 && minor >= 8)
    }
}

/// Block until the given fd is readable via `poll(2)`.
/// Returns `true` when ready, `false` on unrecoverable error.
pub fn poll_readable(fd: i32) -> bool {
    let mut pfd = libc::pollfd {
        fd,
        events: libc::POLLIN,
        revents: 0,
    };
    loop {
        let ret = unsafe { libc::poll(&raw mut pfd, 1, -1) };
        if ret >= 0 {
            return true;
        }
        let err = std::io::Error::last_os_error();
        if err.kind() == std::io::ErrorKind::Interrupted {
            continue;
        }
        warn!("poll error on fd {fd}: {err}");
        return false;
    }
}
