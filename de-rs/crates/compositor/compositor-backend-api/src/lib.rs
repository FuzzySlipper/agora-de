#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BackendCapability {
    KernelCredentialAttribution,
    SurfaceLifecycleEvents,
    SynchronousInputDeny,
    PerToplevelCapture,
    GeometryControl,
}

impl BackendCapability {
    pub fn key(self) -> &'static str {
        match self {
            BackendCapability::KernelCredentialAttribution => "kernel_credential_attribution",
            BackendCapability::SurfaceLifecycleEvents => "surface_lifecycle_events",
            BackendCapability::SynchronousInputDeny => "synchronous_input_deny",
            BackendCapability::PerToplevelCapture => "per_toplevel_capture",
            BackendCapability::GeometryControl => "geometry_control",
        }
    }

    pub fn label(self) -> &'static str {
        match self {
            BackendCapability::KernelCredentialAttribution => "Kernel credential attribution",
            BackendCapability::SurfaceLifecycleEvents => "Surface lifecycle events",
            BackendCapability::SynchronousInputDeny => "Synchronous input deny",
            BackendCapability::PerToplevelCapture => "Per-toplevel capture",
            BackendCapability::GeometryControl => "Geometry control",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CapabilitySupport {
    Native,
    StandardProtocol,
    CustomPlugin,
    Missing,
}

impl CapabilitySupport {
    pub fn key(&self) -> &'static str {
        match self {
            CapabilitySupport::Native => "native",
            CapabilitySupport::StandardProtocol => "standard_protocol",
            CapabilitySupport::CustomPlugin => "custom_plugin",
            CapabilitySupport::Missing => "missing",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CapabilityReport {
    pub capability: BackendCapability,
    pub support: CapabilitySupport,
    pub evidence: &'static str,
}

pub trait CompositorBackend {
    fn name(&self) -> &'static str;
    fn capabilities(&self) -> &'static [BackendCapability];
}

pub fn missing_capabilities(
    required: &[BackendCapability],
    available: &[BackendCapability],
) -> Vec<BackendCapability> {
    required
        .iter()
        .copied()
        .filter(|capability| !available.contains(capability))
        .collect()
}

#[cfg(test)]
mod tests {
    use super::{missing_capabilities, BackendCapability};

    #[test]
    fn reports_missing_capabilities() {
        let required = [
            BackendCapability::KernelCredentialAttribution,
            BackendCapability::SynchronousInputDeny,
        ];
        let available = [BackendCapability::KernelCredentialAttribution];

        assert_eq!(
            missing_capabilities(&required, &available),
            vec![BackendCapability::SynchronousInputDeny]
        );
    }
}
