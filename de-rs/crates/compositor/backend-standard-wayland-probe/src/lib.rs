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
        evidence: "probe-observations: security-context-v1/wp_security_context_v1 scoped sockets attach sandbox/app identity",
    },
    CapabilityReport {
        capability: BackendCapability::SurfaceLifecycleEvents,
        support: CapabilitySupport::StandardProtocol,
        evidence: "probe-observations: ext-foreign-toplevel-list-v1 provides foreign toplevel handles",
    },
    CapabilityReport {
        capability: BackendCapability::SynchronousInputDeny,
        support: CapabilitySupport::Missing,
        evidence: "probe-observations: no standard protocol provides synchronous per-surface input deny in the compositor input path",
    },
    CapabilityReport {
        capability: BackendCapability::PerToplevelCapture,
        support: CapabilitySupport::StandardProtocol,
        evidence: "probe-observations: ext-image-copy-capture-v1 captures image sources including toplevels into client buffers",
    },
    CapabilityReport {
        capability: BackendCapability::GeometryControl,
        support: CapabilitySupport::Missing,
        evidence: "probe-observations: no standard protocol grants DE-controlled per-surface move/resize authority",
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

    #[test]
    fn report_evidence_points_to_checked_probe_observations() {
        for entry in STANDARD_PROTOCOL_REPORT {
            assert!(
                entry.evidence.starts_with("probe-observations:"),
                "report evidence for {} must cite probe-observations",
                entry.capability.key()
            );
        }
    }
}
