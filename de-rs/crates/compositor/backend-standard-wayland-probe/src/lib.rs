use compositor_backend_api::{BackendCapability, CapabilityReport, CapabilitySupport};

pub const PROBE_NAME: &str = "standard-wayland-protocol-probe";

pub const STANDARD_PROTOCOL_CAPABILITIES: &[BackendCapability] = &[
    BackendCapability::KernelCredentialAttribution,
    BackendCapability::SurfaceLifecycleEvents,
    BackendCapability::PerToplevelCapture,
];

pub const STANDARD_PROTOCOL_REPORT: &[CapabilityReport] = &[
    CapabilityReport {
        capability: BackendCapability::KernelCredentialAttribution,
        support: CapabilitySupport::StandardProtocol,
        evidence: "wp_security_context_v1 scoped Wayland sockets",
    },
    CapabilityReport {
        capability: BackendCapability::SurfaceLifecycleEvents,
        support: CapabilitySupport::StandardProtocol,
        evidence: "ext_foreign_toplevel_list_v1 / foreign toplevel management",
    },
    CapabilityReport {
        capability: BackendCapability::SynchronousInputDeny,
        support: CapabilitySupport::Missing,
        evidence: "no known standard protocol for synchronous per-surface input deny",
    },
    CapabilityReport {
        capability: BackendCapability::PerToplevelCapture,
        support: CapabilitySupport::StandardProtocol,
        evidence: "ext_image_copy_capture_v1 per-toplevel capture",
    },
    CapabilityReport {
        capability: BackendCapability::GeometryControl,
        support: CapabilitySupport::Missing,
        evidence: "per-surface move/resize geometry remains compositor-specific",
    },
];

#[cfg(test)]
mod tests {
    use compositor_backend_api::{missing_capabilities, BackendCapability};

    use super::{STANDARD_PROTOCOL_CAPABILITIES, STANDARD_PROTOCOL_REPORT};

    #[test]
    fn standard_protocol_probe_keeps_input_deny_explicitly_missing() {
        let required = [
            BackendCapability::KernelCredentialAttribution,
            BackendCapability::SurfaceLifecycleEvents,
            BackendCapability::SynchronousInputDeny,
            BackendCapability::PerToplevelCapture,
            BackendCapability::GeometryControl,
        ];

        let missing = missing_capabilities(&required, STANDARD_PROTOCOL_CAPABILITIES);

        assert!(missing.contains(&BackendCapability::SynchronousInputDeny));
        assert!(missing.contains(&BackendCapability::GeometryControl));
    }

    #[test]
    fn report_explains_every_required_capability() {
        let required = [
            BackendCapability::KernelCredentialAttribution,
            BackendCapability::SurfaceLifecycleEvents,
            BackendCapability::SynchronousInputDeny,
            BackendCapability::PerToplevelCapture,
            BackendCapability::GeometryControl,
        ];

        for capability in required {
            assert!(
                STANDARD_PROTOCOL_REPORT
                    .iter()
                    .any(|entry| entry.capability == capability),
                "missing report entry for {}",
                capability.key()
            );
        }
    }
}
