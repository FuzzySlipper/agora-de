use compositor_backend_api::BackendCapability;

pub const WAYFIRE_CAPABILITIES: &[BackendCapability] = &[
    BackendCapability::KernelCredentialAttribution,
    BackendCapability::SurfaceLifecycleEvents,
    BackendCapability::SynchronousInputDeny,
    BackendCapability::PerToplevelCapture,
    BackendCapability::GeometryControl,
];

