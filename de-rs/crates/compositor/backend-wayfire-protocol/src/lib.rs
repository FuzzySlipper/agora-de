use compositor_backend_api::{BackendCapability, CapabilityReport, CapabilitySupport};

pub const WAYFIRE_CAPABILITIES: &[BackendCapability] = &[
    BackendCapability::KernelCredentialAttribution,
    BackendCapability::SurfaceLifecycleEvents,
    BackendCapability::SynchronousInputDeny,
    BackendCapability::PerToplevelCapture,
    BackendCapability::GeometryControl,
];

pub const WAYFIRE_REPORT: &[CapabilityReport] = &[
    CapabilityReport {
        capability: BackendCapability::KernelCredentialAttribution,
        support: CapabilitySupport::CustomPlugin,
        evidence: "wl_client_get_credentials via Wayfire plugin",
    },
    CapabilityReport {
        capability: BackendCapability::SurfaceLifecycleEvents,
        support: CapabilitySupport::CustomPlugin,
        evidence: "Wayfire view lifecycle signals via plugin socket",
    },
    CapabilityReport {
        capability: BackendCapability::SynchronousInputDeny,
        support: CapabilitySupport::CustomPlugin,
        evidence: "local policy cache in compositor input path",
    },
    CapabilityReport {
        capability: BackendCapability::PerToplevelCapture,
        support: CapabilitySupport::CustomPlugin,
        evidence: "predecessor GLES readback path",
    },
    CapabilityReport {
        capability: BackendCapability::GeometryControl,
        support: CapabilitySupport::CustomPlugin,
        evidence: "Wayfire view control APIs",
    },
];

#[cfg(test)]
mod tests {
    use super::{WAYFIRE_CAPABILITIES, WAYFIRE_REPORT};

    #[test]
    fn report_covers_every_declared_capability() {
        for capability in WAYFIRE_CAPABILITIES {
            assert!(
                WAYFIRE_REPORT
                    .iter()
                    .any(|entry| entry.capability == *capability),
                "missing report entry for {}",
                capability.key()
            );
        }
    }
}
