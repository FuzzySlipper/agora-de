#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BackendCapability {
    KernelCredentialAttribution,
    SurfaceLifecycleEvents,
    SynchronousInputDeny,
    PerToplevelCapture,
    GeometryControl,
}

pub trait CompositorBackend {
    fn name(&self) -> &'static str;
    fn capabilities(&self) -> &'static [BackendCapability];
}

