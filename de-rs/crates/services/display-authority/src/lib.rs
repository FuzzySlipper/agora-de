use protocol_settings::{
    DisplayApplyRequest, DisplayHeadConfiguration, DisplayHeadIdentity, DisplayMode,
    DisplayProtocolCapabilities, DisplayReconciliationState, DisplayReconciliationStatus,
    DisplaySettingsState, DisplayTopology, DisplayTransform, DisplayValidateRequest,
    SettingsAvailabilityState, SettingsModuleAvailability, SettingsValidationIssue,
    SettingsValidationResponse, DISPLAYS_CONTRACT_VERSION, DISPLAYS_MODULE_ID,
};
use std::collections::BTreeMap;
use wayland_client::{
    globals::{registry_queue_init, GlobalListContents},
    protocol::{wl_output, wl_registry},
    Connection, Dispatch, EventQueue, Proxy, QueueHandle, WEnum,
};
use wayland_protocols_wlr::output_management::v1::client::{
    zwlr_output_configuration_head_v1::ZwlrOutputConfigurationHeadV1,
    zwlr_output_configuration_v1::{self, ZwlrOutputConfigurationV1},
    zwlr_output_head_v1::{self, ZwlrOutputHeadV1},
    zwlr_output_manager_v1::{self, ZwlrOutputManagerV1},
    zwlr_output_mode_v1::{self, ZwlrOutputModeV1},
};

mod persistence;
mod transaction;

pub use persistence::{
    acquire_transaction_lock, load_profile, load_transaction, persist_profile, persist_transaction,
    release_transaction_lock, transaction_lock_path, unix_millis, DisplayProfile,
    StoredTransaction, DISPLAY_PROFILE_SCHEMA_VERSION,
};
pub use transaction::{LeaseAction, LeaseEvent, LeaseMachine, LeasePhase};

const MAX_OUTPUT_MANAGER_VERSION: u32 = 4;

#[derive(Clone, Debug, Eq, PartialEq)]
struct ModeDraft {
    width: i32,
    height: i32,
    refresh_millihz: i32,
    preferred: bool,
    connected: bool,
}

impl Default for ModeDraft {
    fn default() -> Self {
        Self {
            width: 0,
            height: 0,
            refresh_millihz: 0,
            preferred: false,
            connected: true,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct HeadDraft {
    identity: DisplayHeadIdentity,
    connected: bool,
    enabled: bool,
    mode_keys: Vec<u32>,
    current_mode_key: Option<u32>,
    x: i32,
    y: i32,
    scale_milli: u32,
    transform: DisplayTransform,
    adaptive_sync: bool,
}

impl Default for HeadDraft {
    fn default() -> Self {
        Self {
            identity: DisplayHeadIdentity {
                name: String::new(),
                description: String::new(),
                make: String::new(),
                model: String::new(),
                serial_number: String::new(),
                physical_width_mm: 0,
                physical_height_mm: 0,
            },
            connected: true,
            enabled: false,
            mode_keys: Vec::new(),
            current_mode_key: None,
            x: 0,
            y: 0,
            scale_milli: 1000,
            transform: DisplayTransform::Normal,
            adaptive_sync: false,
        }
    }
}

#[derive(Clone, Debug)]
pub struct DiscoveryAccumulator {
    protocol_version: u32,
    heads: BTreeMap<u32, HeadDraft>,
    modes: BTreeMap<u32, ModeDraft>,
    mode_heads: BTreeMap<u32, u32>,
    revision: u64,
    coherent: Option<DisplayTopology>,
    manager_finished: bool,
}

impl DiscoveryAccumulator {
    pub fn new(protocol_version: u32) -> Self {
        Self {
            protocol_version,
            heads: BTreeMap::new(),
            modes: BTreeMap::new(),
            mode_heads: BTreeMap::new(),
            revision: 0,
            coherent: None,
            manager_finished: false,
        }
    }

    pub fn introduce_head(&mut self, key: u32) {
        self.heads.entry(key).or_default();
    }

    pub fn introduce_mode(&mut self, head_key: u32, mode_key: u32) {
        self.heads
            .entry(head_key)
            .or_default()
            .mode_keys
            .push(mode_key);
        self.modes.entry(mode_key).or_default();
        self.mode_heads.insert(mode_key, head_key);
    }

    pub fn set_head_name(&mut self, key: u32, value: String) {
        self.heads.entry(key).or_default().identity.name = value;
    }

    pub fn set_head_description(&mut self, key: u32, value: String) {
        self.heads.entry(key).or_default().identity.description = value;
    }

    pub fn set_head_make(&mut self, key: u32, value: String) {
        self.heads.entry(key).or_default().identity.make = value;
    }

    pub fn set_head_model(&mut self, key: u32, value: String) {
        self.heads.entry(key).or_default().identity.model = value;
    }

    pub fn set_head_serial(&mut self, key: u32, value: String) {
        self.heads.entry(key).or_default().identity.serial_number = value;
    }

    pub fn set_physical_size(&mut self, key: u32, width: i32, height: i32) {
        let head = self.heads.entry(key).or_default();
        head.identity.physical_width_mm = width;
        head.identity.physical_height_mm = height;
    }

    pub fn set_enabled(&mut self, key: u32, enabled: bool) {
        self.heads.entry(key).or_default().enabled = enabled;
    }

    pub fn set_current_mode(&mut self, key: u32, mode_key: u32) {
        self.heads.entry(key).or_default().current_mode_key = Some(mode_key);
    }

    pub fn set_position(&mut self, key: u32, x: i32, y: i32) {
        let head = self.heads.entry(key).or_default();
        head.x = x;
        head.y = y;
    }

    pub fn set_scale_milli(&mut self, key: u32, scale_milli: u32) {
        self.heads.entry(key).or_default().scale_milli = scale_milli;
    }

    pub fn set_transform(&mut self, key: u32, transform: DisplayTransform) {
        self.heads.entry(key).or_default().transform = transform;
    }

    pub fn set_adaptive_sync(&mut self, key: u32, enabled: bool) {
        self.heads.entry(key).or_default().adaptive_sync = enabled;
    }

    pub fn set_mode_size(&mut self, key: u32, width: i32, height: i32) {
        let mode = self.modes.entry(key).or_default();
        mode.width = width;
        mode.height = height;
    }

    pub fn set_mode_refresh(&mut self, key: u32, refresh_millihz: i32) {
        self.modes.entry(key).or_default().refresh_millihz = refresh_millihz;
    }

    pub fn set_mode_preferred(&mut self, key: u32) {
        self.modes.entry(key).or_default().preferred = true;
    }

    pub fn finish_mode(&mut self, key: u32) {
        if let Some(mode) = self.modes.get_mut(&key) {
            mode.connected = false;
        }
    }

    pub fn finish_head(&mut self, key: u32) {
        if let Some(head) = self.heads.get_mut(&key) {
            head.connected = false;
            head.enabled = false;
            head.current_mode_key = None;
        }
    }

    pub fn finish_manager(&mut self) {
        self.manager_finished = true;
    }

    pub fn commit(&mut self, serial: u32) {
        let topology = self.build_topology(serial);
        if self.coherent.as_ref() != Some(&topology) {
            self.revision = self.revision.saturating_add(1);
            self.coherent = Some(topology);
        }
    }

    pub fn state(&self) -> DisplaySettingsState {
        if self.manager_finished {
            return unavailable_state("output-management manager finished");
        }
        let Some(active) = self.coherent.clone() else {
            return unavailable_state("waiting for a coherent output-management snapshot");
        };
        DisplaySettingsState {
            module_id: DISPLAYS_MODULE_ID.to_string(),
            contract_version: DISPLAYS_CONTRACT_VERSION,
            revision: active.serial as u64,
            defaults: preferred_defaults(&active),
            active,
            capabilities: DisplayProtocolCapabilities {
                output_management: true,
                protocol_version: self.protocol_version,
                test_configuration: true,
                apply_configuration: true,
                adaptive_sync: self.protocol_version >= 4,
            },
            lease: None,
            reconciliation: DisplayReconciliationStatus {
                state: DisplayReconciliationState::NotNeeded,
                detail: "no reconciliation required".to_string(),
                matched_heads: Vec::new(),
                unmatched_profile_heads: Vec::new(),
            },
            availability: SettingsModuleAvailability {
                state: SettingsAvailabilityState::Available,
                reason: None,
            },
        }
    }

    fn build_topology(&self, serial: u32) -> DisplayTopology {
        let heads = self
            .heads
            .iter()
            .map(|(head_key, head)| {
                let mut modes = Vec::new();
                let mut mode_ids = BTreeMap::new();
                let mut mode_occurrences = BTreeMap::<String, usize>::new();
                for mode_key in &head.mode_keys {
                    let Some(mode) = self.modes.get(mode_key) else {
                        continue;
                    };
                    if !mode.connected {
                        continue;
                    }
                    let base_id = mode_id(mode);
                    let occurrence = mode_occurrences.entry(base_id.clone()).or_default();
                    *occurrence += 1;
                    let id = if *occurrence == 1 {
                        base_id
                    } else {
                        format!("{base_id}#{}", *occurrence)
                    };
                    mode_ids.insert(*mode_key, id.clone());
                    modes.push(DisplayMode {
                        id,
                        width: mode.width,
                        height: mode.height,
                        refresh_millihz: mode.refresh_millihz,
                        preferred: mode.preferred,
                    });
                }
                modes.sort_by(|left, right| {
                    (left.width, left.height, left.refresh_millihz).cmp(&(
                        right.width,
                        right.height,
                        right.refresh_millihz,
                    ))
                });
                let id = if head.identity.name.is_empty() {
                    format!("head-{head_key}")
                } else {
                    head.identity.name.clone()
                };
                DisplayHeadConfiguration {
                    id,
                    identity: head.identity.clone(),
                    connected: head.connected,
                    enabled: head.enabled,
                    modes,
                    current_mode_id: head
                        .current_mode_key
                        .and_then(|mode_key| mode_ids.get(&mode_key).cloned()),
                    x: head.x,
                    y: head.y,
                    scale_milli: head.scale_milli,
                    transform: head.transform,
                    adaptive_sync: head.adaptive_sync,
                }
            })
            .collect();
        DisplayTopology { serial, heads }
    }
}

fn mode_id(mode: &ModeDraft) -> String {
    format!("{}x{}@{}", mode.width, mode.height, mode.refresh_millihz)
}

fn preferred_defaults(active: &DisplayTopology) -> DisplayTopology {
    let mut defaults = active.clone();
    for head in &mut defaults.heads {
        if let Some(preferred) = head.modes.iter().find(|mode| mode.preferred) {
            head.current_mode_id = Some(preferred.id.clone());
        }
        head.scale_milli = 1000;
        head.transform = DisplayTransform::Normal;
        head.adaptive_sync = false;
    }
    defaults
}

/// Rebase a previously known-good topology onto a fresh compositor snapshot.
/// Live proxy and mode identifiers always come from `current`; only settings
/// for heads that still match by stable identity are projected across.
pub fn rebase_topology(desired: &DisplayTopology, current: &DisplayTopology) -> DisplayTopology {
    let mut rebased = current.clone();
    for head in &mut rebased.heads {
        let Some(saved) = desired.heads.iter().find(|candidate| {
            candidate.id == head.id
                && candidate.identity.make == head.identity.make
                && candidate.identity.model == head.identity.model
                && candidate.identity.serial_number == head.identity.serial_number
        }) else {
            continue;
        };
        head.enabled = saved.enabled;
        head.x = saved.x;
        head.y = saved.y;
        head.scale_milli = saved.scale_milli;
        head.transform = saved.transform;
        head.adaptive_sync = saved.adaptive_sync;
        head.current_mode_id = saved.current_mode_id.as_deref().and_then(|saved_id| {
            let saved_mode = saved.modes.iter().find(|mode| mode.id == saved_id)?;
            head.modes
                .iter()
                .find(|mode| {
                    mode.width == saved_mode.width
                        && mode.height == saved_mode.height
                        && mode.refresh_millihz == saved_mode.refresh_millihz
                })
                .map(|mode| mode.id.clone())
        });
        if head.enabled && head.current_mode_id.is_none() {
            head.current_mode_id = head
                .modes
                .iter()
                .find(|mode| mode.preferred)
                .or_else(|| head.modes.first())
                .map(|mode| mode.id.clone());
        }
    }
    if !rebased
        .heads
        .iter()
        .any(|head| head.connected && head.enabled)
    {
        if let Some(head) = rebased.heads.iter_mut().find(|head| head.connected) {
            head.enabled = true;
            head.current_mode_id = head
                .modes
                .iter()
                .find(|mode| mode.preferred)
                .or_else(|| head.modes.first())
                .map(|mode| mode.id.clone());
            head.x = 0;
            head.y = 0;
            head.scale_milli = 1000;
            head.transform = DisplayTransform::Normal;
        }
    }
    rebased
}

pub fn reconcile_profile(
    profile: &DisplayTopology,
    current: &DisplayTopology,
) -> (DisplayTopology, DisplayReconciliationStatus) {
    let mut target = current.clone();
    let mut matched_heads = Vec::new();
    let mut unmatched_profile_heads = Vec::new();
    let mut degraded = false;
    for saved in &profile.heads {
        let matches = target
            .heads
            .iter()
            .enumerate()
            .filter(|(_, candidate)| same_physical_display(saved, candidate))
            .map(|(index, _)| index)
            .collect::<Vec<_>>();
        if matches.len() != 1 {
            unmatched_profile_heads.push(saved.id.clone());
            continue;
        }
        let target_head = &mut target.heads[matches[0]];
        matched_heads.push(target_head.id.clone());
        target_head.enabled = saved.enabled;
        target_head.x = saved.x;
        target_head.y = saved.y;
        target_head.scale_milli = saved.scale_milli;
        target_head.transform = saved.transform;
        target_head.adaptive_sync = saved.adaptive_sync;
        target_head.current_mode_id = saved.current_mode_id.as_deref().and_then(|saved_id| {
            let saved_mode = saved.modes.iter().find(|mode| mode.id == saved_id)?;
            target_head
                .modes
                .iter()
                .find(|mode| {
                    mode.width == saved_mode.width
                        && mode.height == saved_mode.height
                        && mode.refresh_millihz == saved_mode.refresh_millihz
                })
                .map(|mode| mode.id.clone())
        });
        if target_head.enabled && target_head.current_mode_id.is_none() {
            degraded = true;
            target_head.current_mode_id = target_head
                .modes
                .iter()
                .find(|mode| mode.preferred)
                .or_else(|| target_head.modes.first())
                .map(|mode| mode.id.clone());
        }
    }
    let mut safe_fallback = false;
    if !target
        .heads
        .iter()
        .any(|head| head.connected && head.enabled && head.current_mode_id.is_some())
    {
        safe_fallback = true;
        if let Some(head) = target.heads.iter_mut().find(|head| head.connected) {
            head.enabled = true;
            head.current_mode_id = head
                .modes
                .iter()
                .find(|mode| mode.preferred)
                .or_else(|| head.modes.first())
                .map(|mode| mode.id.clone());
            head.x = 0;
            head.y = 0;
            head.scale_milli = 1000;
            head.transform = DisplayTransform::Normal;
        }
    }
    let (state, detail) = if safe_fallback {
        (
            DisplayReconciliationState::SafeFallback,
            "profile could not produce a usable topology; enabled a safe preferred output",
        )
    } else if !unmatched_profile_heads.is_empty() || degraded {
        (
            DisplayReconciliationState::PartiallyMatched,
            "applied the unambiguous portion of the confirmed display profile",
        )
    } else if target == *current {
        (
            DisplayReconciliationState::NotNeeded,
            "confirmed profile already matches connected displays",
        )
    } else {
        (
            DisplayReconciliationState::Applied,
            "confirmed display profile matched connected displays",
        )
    };
    (
        target,
        DisplayReconciliationStatus {
            state,
            detail: detail.to_string(),
            matched_heads,
            unmatched_profile_heads,
        },
    )
}

fn same_physical_display(
    saved: &DisplayHeadConfiguration,
    current: &DisplayHeadConfiguration,
) -> bool {
    if !saved.identity.serial_number.is_empty() && !current.identity.serial_number.is_empty() {
        return saved.identity.make == current.identity.make
            && saved.identity.model == current.identity.model
            && saved.identity.serial_number == current.identity.serial_number;
    }
    saved.id == current.id
        && saved.identity.make == current.identity.make
        && saved.identity.model == current.identity.model
}

pub fn unavailable_state(reason: &str) -> DisplaySettingsState {
    DisplaySettingsState {
        module_id: DISPLAYS_MODULE_ID.to_string(),
        contract_version: DISPLAYS_CONTRACT_VERSION,
        revision: 0,
        active: DisplayTopology {
            serial: 0,
            heads: Vec::new(),
        },
        defaults: DisplayTopology {
            serial: 0,
            heads: Vec::new(),
        },
        capabilities: DisplayProtocolCapabilities {
            output_management: false,
            protocol_version: 0,
            test_configuration: false,
            apply_configuration: false,
            adaptive_sync: false,
        },
        lease: None,
        reconciliation: DisplayReconciliationStatus {
            state: DisplayReconciliationState::Failed,
            detail: reason.to_string(),
            matched_heads: Vec::new(),
            unmatched_profile_heads: Vec::new(),
        },
        availability: SettingsModuleAvailability {
            state: SettingsAvailabilityState::Unavailable,
            reason: Some(reason.to_string()),
        },
    }
}

struct WaylandState {
    accumulator: DiscoveryAccumulator,
    got_done: bool,
    head_proxies: BTreeMap<u32, ZwlrOutputHeadV1>,
    mode_proxies: BTreeMap<u32, ZwlrOutputModeV1>,
    configuration_result: Option<ConfigurationResult>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum ConfigurationResult {
    Succeeded,
    Failed,
    Cancelled,
}

impl Dispatch<wl_registry::WlRegistry, GlobalListContents> for WaylandState {
    fn event(
        _: &mut Self,
        _: &wl_registry::WlRegistry,
        _: wl_registry::Event,
        _: &GlobalListContents,
        _: &Connection,
        _: &QueueHandle<Self>,
    ) {
    }
}

impl Dispatch<ZwlrOutputManagerV1, ()> for WaylandState {
    fn event(
        state: &mut Self,
        _: &ZwlrOutputManagerV1,
        event: zwlr_output_manager_v1::Event,
        _: &(),
        _: &Connection,
        _: &QueueHandle<Self>,
    ) {
        match event {
            zwlr_output_manager_v1::Event::Head { head } => {
                let key = head.id().protocol_id();
                state.accumulator.introduce_head(key);
                state.head_proxies.insert(key, head);
            }
            zwlr_output_manager_v1::Event::Done { serial } => {
                state.accumulator.commit(serial);
                state.got_done = true;
            }
            zwlr_output_manager_v1::Event::Finished => state.accumulator.finish_manager(),
            _ => {}
        }
    }

    wayland_client::event_created_child!(WaylandState, ZwlrOutputManagerV1, [
        zwlr_output_manager_v1::EVT_HEAD_OPCODE => (ZwlrOutputHeadV1, ())
    ]);
}

impl Dispatch<ZwlrOutputHeadV1, ()> for WaylandState {
    fn event(
        state: &mut Self,
        proxy: &ZwlrOutputHeadV1,
        event: zwlr_output_head_v1::Event,
        _: &(),
        _: &Connection,
        _: &QueueHandle<Self>,
    ) {
        let key = proxy.id().protocol_id();
        match event {
            zwlr_output_head_v1::Event::Name { name } => state.accumulator.set_head_name(key, name),
            zwlr_output_head_v1::Event::Description { description } => {
                state.accumulator.set_head_description(key, description);
            }
            zwlr_output_head_v1::Event::PhysicalSize { width, height } => {
                state.accumulator.set_physical_size(key, width, height);
            }
            zwlr_output_head_v1::Event::Mode { mode } => {
                let mode_key = mode.id().protocol_id();
                state.accumulator.introduce_mode(key, mode_key);
                state.mode_proxies.insert(mode_key, mode);
            }
            zwlr_output_head_v1::Event::Enabled { enabled } => {
                state.accumulator.set_enabled(key, enabled != 0);
            }
            zwlr_output_head_v1::Event::CurrentMode { mode } => {
                state
                    .accumulator
                    .set_current_mode(key, mode.id().protocol_id());
            }
            zwlr_output_head_v1::Event::Position { x, y } => {
                state.accumulator.set_position(key, x, y);
            }
            zwlr_output_head_v1::Event::Transform { transform } => {
                state
                    .accumulator
                    .set_transform(key, transform_from_wire(transform));
            }
            zwlr_output_head_v1::Event::Scale { scale } => {
                state
                    .accumulator
                    .set_scale_milli(key, (scale * 1000.0).round().max(1.0) as u32);
            }
            zwlr_output_head_v1::Event::Finished => state.accumulator.finish_head(key),
            zwlr_output_head_v1::Event::Make { make } => state.accumulator.set_head_make(key, make),
            zwlr_output_head_v1::Event::Model { model } => {
                state.accumulator.set_head_model(key, model);
            }
            zwlr_output_head_v1::Event::SerialNumber { serial_number } => {
                state.accumulator.set_head_serial(key, serial_number);
            }
            zwlr_output_head_v1::Event::AdaptiveSync { state: adaptive } => {
                state.accumulator.set_adaptive_sync(
                    key,
                    matches!(
                        adaptive,
                        WEnum::Value(zwlr_output_head_v1::AdaptiveSyncState::Enabled)
                    ),
                );
            }
            _ => {}
        }
    }

    wayland_client::event_created_child!(WaylandState, ZwlrOutputHeadV1, [
        zwlr_output_head_v1::EVT_MODE_OPCODE => (ZwlrOutputModeV1, ())
    ]);
}

impl Dispatch<ZwlrOutputConfigurationV1, ()> for WaylandState {
    fn event(
        state: &mut Self,
        _: &ZwlrOutputConfigurationV1,
        event: zwlr_output_configuration_v1::Event,
        _: &(),
        _: &Connection,
        _: &QueueHandle<Self>,
    ) {
        state.configuration_result = match event {
            zwlr_output_configuration_v1::Event::Succeeded => Some(ConfigurationResult::Succeeded),
            zwlr_output_configuration_v1::Event::Failed => Some(ConfigurationResult::Failed),
            zwlr_output_configuration_v1::Event::Cancelled => Some(ConfigurationResult::Cancelled),
            _ => state.configuration_result,
        };
    }
}

impl Dispatch<ZwlrOutputConfigurationHeadV1, ()> for WaylandState {
    fn event(
        _: &mut Self,
        _: &ZwlrOutputConfigurationHeadV1,
        _: <ZwlrOutputConfigurationHeadV1 as Proxy>::Event,
        _: &(),
        _: &Connection,
        _: &QueueHandle<Self>,
    ) {
    }
}

impl Dispatch<ZwlrOutputModeV1, ()> for WaylandState {
    fn event(
        state: &mut Self,
        proxy: &ZwlrOutputModeV1,
        event: zwlr_output_mode_v1::Event,
        _: &(),
        _: &Connection,
        _: &QueueHandle<Self>,
    ) {
        let key = proxy.id().protocol_id();
        match event {
            zwlr_output_mode_v1::Event::Size { width, height } => {
                state.accumulator.set_mode_size(key, width, height);
            }
            zwlr_output_mode_v1::Event::Refresh { refresh } => {
                state.accumulator.set_mode_refresh(key, refresh);
            }
            zwlr_output_mode_v1::Event::Preferred => state.accumulator.set_mode_preferred(key),
            zwlr_output_mode_v1::Event::Finished => state.accumulator.finish_mode(key),
            _ => {}
        }
    }
}

fn transform_from_wire(transform: WEnum<wl_output::Transform>) -> DisplayTransform {
    match transform {
        WEnum::Value(wl_output::Transform::_90) => DisplayTransform::Rotate90,
        WEnum::Value(wl_output::Transform::_180) => DisplayTransform::Rotate180,
        WEnum::Value(wl_output::Transform::_270) => DisplayTransform::Rotate270,
        WEnum::Value(wl_output::Transform::Flipped) => DisplayTransform::Flipped,
        WEnum::Value(wl_output::Transform::Flipped90) => DisplayTransform::Flipped90,
        WEnum::Value(wl_output::Transform::Flipped180) => DisplayTransform::Flipped180,
        WEnum::Value(wl_output::Transform::Flipped270) => DisplayTransform::Flipped270,
        _ => DisplayTransform::Normal,
    }
}

struct WaylandRuntime {
    _connection: Connection,
    event_queue: EventQueue<WaylandState>,
    manager: ZwlrOutputManagerV1,
    state: WaylandState,
}

impl WaylandRuntime {
    fn connect() -> Result<Self, String> {
        let connection = Connection::connect_to_env()
            .map_err(|error| format!("connect to Wayland compositor: {error}"))?;
        let (globals, mut event_queue) = registry_queue_init::<WaylandState>(&connection)
            .map_err(|error| format!("read Wayland globals: {error}"))?;
        let manager: ZwlrOutputManagerV1 = globals
            .bind(&event_queue.handle(), 1..=MAX_OUTPUT_MANAGER_VERSION, ())
            .map_err(|error| format!("bind zwlr_output_manager_v1: {error}"))?;
        let mut state = WaylandState {
            accumulator: DiscoveryAccumulator::new(manager.version()),
            got_done: false,
            head_proxies: BTreeMap::new(),
            mode_proxies: BTreeMap::new(),
            configuration_result: None,
        };
        for _ in 0..4 {
            event_queue
                .roundtrip(&mut state)
                .map_err(|error| format!("read output-management snapshot: {error}"))?;
            if state.got_done || state.accumulator.manager_finished {
                break;
            }
        }
        Ok(Self {
            _connection: connection,
            event_queue,
            manager,
            state,
        })
    }
}

pub fn discover_from_environment() -> DisplaySettingsState {
    match WaylandRuntime::connect() {
        Ok(runtime) => runtime.state.accumulator.state(),
        Err(error) => unavailable_state(&error),
    }
}

pub fn validate_topology(
    state: &DisplaySettingsState,
    request: &DisplayValidateRequest,
) -> SettingsValidationResponse {
    let mut issues = Vec::new();
    if request.contract_version != DISPLAYS_CONTRACT_VERSION {
        issue(
            &mut issues,
            "contractVersion",
            "unsupported_contract_version",
            "The Displays contract version is unsupported.",
        );
    }
    if request.base_revision != state.revision || request.draft.serial != state.active.serial {
        issue(
            &mut issues,
            "baseRevision",
            "stale_revision",
            "Display state changed; reload before applying.",
        );
    }
    let active_heads = state
        .active
        .heads
        .iter()
        .filter(|head| head.connected)
        .map(|head| (head.id.as_str(), head))
        .collect::<BTreeMap<_, _>>();
    let draft_heads = request
        .draft
        .heads
        .iter()
        .map(|head| (head.id.as_str(), head))
        .collect::<BTreeMap<_, _>>();
    if active_heads.keys().ne(draft_heads.keys()) {
        issue(
            &mut issues,
            "heads",
            "head_set_changed",
            "The complete connected head set must be configured.",
        );
    }
    let mut usable = 0;
    let mut rectangles = Vec::new();
    for head in &request.draft.heads {
        let field = format!("heads.{}", head.id);
        let Some(active) = active_heads.get(head.id.as_str()) else {
            issue(
                &mut issues,
                &field,
                "unknown_head",
                "The display is no longer connected.",
            );
            continue;
        };
        if !(500..=4000).contains(&head.scale_milli) {
            issue(
                &mut issues,
                &format!("{field}.scaleMilli"),
                "invalid_scale",
                "Scale must be between 50% and 400%.",
            );
        }
        if !(-32_768..=32_768).contains(&head.x) || !(-32_768..=32_768).contains(&head.y) {
            issue(
                &mut issues,
                &format!("{field}.position"),
                "invalid_position",
                "Logical position is outside the supported range.",
            );
        }
        if !head.enabled {
            continue;
        }
        let Some(mode_id) = head.current_mode_id.as_deref() else {
            issue(
                &mut issues,
                &format!("{field}.currentModeId"),
                "missing_mode",
                "An enabled display requires a mode.",
            );
            continue;
        };
        let Some(mode) = active.modes.iter().find(|mode| mode.id == mode_id) else {
            issue(
                &mut issues,
                &format!("{field}.currentModeId"),
                "unknown_mode",
                "The selected mode is no longer available.",
            );
            continue;
        };
        usable += 1;
        let (physical_width, physical_height) = match head.transform {
            DisplayTransform::Rotate90
            | DisplayTransform::Rotate270
            | DisplayTransform::Flipped90
            | DisplayTransform::Flipped270 => (mode.height, mode.width),
            _ => (mode.width, mode.height),
        };
        let width = (i64::from(physical_width) * 1000 / i64::from(head.scale_milli)) as i32;
        let height = (i64::from(physical_height) * 1000 / i64::from(head.scale_milli)) as i32;
        rectangles.push((head.id.as_str(), head.x, head.y, width, height));
    }
    if usable == 0 {
        issue(
            &mut issues,
            "heads",
            "no_usable_output",
            "At least one connected output must remain enabled with a valid mode.",
        );
    }
    for left in 0..rectangles.len() {
        for right in left + 1..rectangles.len() {
            let (left_id, left_x, left_y, left_width, left_height) = rectangles[left];
            let (right_id, right_x, right_y, right_width, right_height) = rectangles[right];
            let overlaps = left_x < right_x + right_width
                && left_x + left_width > right_x
                && left_y < right_y + right_height
                && left_y + left_height > right_y;
            if overlaps {
                issue(
                    &mut issues,
                    "heads",
                    "overlapping_outputs",
                    &format!("Displays {left_id} and {right_id} overlap."),
                );
            }
        }
    }
    SettingsValidationResponse {
        valid: issues.is_empty(),
        issues,
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ConfigurationError {
    Validation(Vec<SettingsValidationIssue>),
    StaleRevision,
    TestFailed,
    ApplyFailed,
    Cancelled,
    Unavailable(String),
}

pub fn test_from_environment(
    request: &DisplayValidateRequest,
) -> Result<DisplaySettingsState, ConfigurationError> {
    configure_from_environment(request, false)
}

pub fn apply_from_environment(
    request: &DisplayApplyRequest,
) -> Result<DisplaySettingsState, ConfigurationError> {
    let validate = DisplayValidateRequest {
        contract_version: request.contract_version,
        base_revision: request.base_revision,
        draft: request.draft.clone(),
    };
    configure_from_environment(&validate, true)
}

fn configure_from_environment(
    request: &DisplayValidateRequest,
    apply: bool,
) -> Result<DisplaySettingsState, ConfigurationError> {
    let mut runtime = WaylandRuntime::connect().map_err(ConfigurationError::Unavailable)?;
    let before = runtime.state.accumulator.state();
    let validation = validate_topology(&before, request);
    if !validation.valid {
        if validation
            .issues
            .iter()
            .any(|issue| issue.code == "stale_revision" || issue.code == "head_set_changed")
        {
            return Err(ConfigurationError::StaleRevision);
        }
        return Err(ConfigurationError::Validation(validation.issues));
    }

    let qh = runtime.event_queue.handle();
    let configuration = runtime
        .manager
        .create_configuration(before.active.serial, &qh, ());
    for draft_head in &request.draft.heads {
        let Some((head_key, active_head)) = runtime
            .state
            .accumulator
            .heads
            .iter()
            .find(|(_, head)| head.identity.name == draft_head.id)
        else {
            return Err(ConfigurationError::StaleRevision);
        };
        let Some(head_proxy) = runtime.state.head_proxies.get(head_key).cloned() else {
            return Err(ConfigurationError::StaleRevision);
        };
        if !draft_head.enabled {
            configuration.disable_head(&head_proxy);
            continue;
        }
        let head_configuration = configuration.enable_head(&head_proxy, &qh, ());
        let selected_mode = draft_head
            .current_mode_id
            .as_deref()
            .and_then(|mode_id| mode_key_for_id(&runtime.state.accumulator, active_head, mode_id));
        let Some(selected_mode) = selected_mode else {
            return Err(ConfigurationError::Validation(vec![
                SettingsValidationIssue {
                    field: format!("heads.{}.currentModeId", draft_head.id),
                    code: "unknown_mode".to_string(),
                    message: "The selected mode is no longer available.".to_string(),
                },
            ]));
        };
        let Some(mode_proxy) = runtime.state.mode_proxies.get(&selected_mode) else {
            return Err(ConfigurationError::StaleRevision);
        };
        head_configuration.set_mode(mode_proxy);
        head_configuration.set_position(draft_head.x, draft_head.y);
        head_configuration.set_transform(transform_to_wire(draft_head.transform));
        head_configuration.set_scale(f64::from(draft_head.scale_milli) / 1000.0);
        if runtime.manager.version() >= 4 {
            let adaptive = if draft_head.adaptive_sync {
                zwlr_output_head_v1::AdaptiveSyncState::Enabled
            } else {
                zwlr_output_head_v1::AdaptiveSyncState::Disabled
            };
            head_configuration.set_adaptive_sync(adaptive);
        }
    }

    runtime.state.configuration_result = None;
    runtime.state.got_done = false;
    if apply {
        configuration.apply();
    } else {
        configuration.test();
    }
    for _ in 0..8 {
        runtime
            .event_queue
            .roundtrip(&mut runtime.state)
            .map_err(|error| ConfigurationError::Unavailable(error.to_string()))?;
        if runtime.state.configuration_result.is_some()
            && (!apply || runtime.state.got_done || runtime.state.accumulator.manager_finished)
        {
            break;
        }
    }
    let result = runtime.state.configuration_result.ok_or_else(|| {
        ConfigurationError::Unavailable("configuration result timed out".to_string())
    })?;
    configuration.destroy();
    match result {
        ConfigurationResult::Succeeded => Ok(runtime.state.accumulator.state()),
        ConfigurationResult::Failed if apply => Err(ConfigurationError::ApplyFailed),
        ConfigurationResult::Failed => Err(ConfigurationError::TestFailed),
        ConfigurationResult::Cancelled => Err(ConfigurationError::Cancelled),
    }
}

fn mode_key_for_id(
    accumulator: &DiscoveryAccumulator,
    head: &HeadDraft,
    requested_id: &str,
) -> Option<u32> {
    let mut occurrences = BTreeMap::<String, usize>::new();
    for mode_key in &head.mode_keys {
        let mode = accumulator.modes.get(mode_key)?;
        if !mode.connected {
            continue;
        }
        let base_id = mode_id(mode);
        let occurrence = occurrences.entry(base_id.clone()).or_default();
        *occurrence += 1;
        let id = if *occurrence == 1 {
            base_id
        } else {
            format!("{base_id}#{}", *occurrence)
        };
        if id == requested_id {
            return Some(*mode_key);
        }
    }
    None
}

fn transform_to_wire(transform: DisplayTransform) -> wl_output::Transform {
    match transform {
        DisplayTransform::Normal => wl_output::Transform::Normal,
        DisplayTransform::Rotate90 => wl_output::Transform::_90,
        DisplayTransform::Rotate180 => wl_output::Transform::_180,
        DisplayTransform::Rotate270 => wl_output::Transform::_270,
        DisplayTransform::Flipped => wl_output::Transform::Flipped,
        DisplayTransform::Flipped90 => wl_output::Transform::Flipped90,
        DisplayTransform::Flipped180 => wl_output::Transform::Flipped180,
        DisplayTransform::Flipped270 => wl_output::Transform::Flipped270,
    }
}

fn issue(issues: &mut Vec<SettingsValidationIssue>, field: &str, code: &str, message: &str) {
    issues.push(SettingsValidationIssue {
        field: field.to_string(),
        code: code.to_string(),
        message: message.to_string(),
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    fn add_mode(
        accumulator: &mut DiscoveryAccumulator,
        head: u32,
        mode: u32,
        width: i32,
        height: i32,
        refresh: i32,
        preferred: bool,
    ) {
        accumulator.introduce_mode(head, mode);
        accumulator.set_mode_size(mode, width, height);
        accumulator.set_mode_refresh(mode, refresh);
        if preferred {
            accumulator.set_mode_preferred(mode);
        }
    }

    #[test]
    fn publishes_only_at_done_and_handles_fractional_scale_transform_and_modes() {
        let mut accumulator = DiscoveryAccumulator::new(4);
        accumulator.introduce_head(1);
        accumulator.set_head_name(1, "HDMI-A-1".to_string());
        accumulator.set_enabled(1, true);
        accumulator.set_position(1, 1920, 0);
        accumulator.set_scale_milli(1, 1250);
        accumulator.set_transform(1, DisplayTransform::Rotate90);
        add_mode(&mut accumulator, 1, 10, 2560, 1440, 60_000, true);
        add_mode(&mut accumulator, 1, 11, 1920, 1080, 59_940, false);
        accumulator.set_current_mode(1, 10);

        assert_eq!(accumulator.state().revision, 0);
        assert_eq!(
            accumulator.state().availability.state,
            SettingsAvailabilityState::Unavailable
        );
        accumulator.commit(44);
        let state = accumulator.state();
        assert_eq!(state.revision, 44);
        assert_eq!(state.active.serial, 44);
        assert_eq!(state.active.heads[0].scale_milli, 1250);
        assert_eq!(state.active.heads[0].transform, DisplayTransform::Rotate90);
        assert_eq!(state.active.heads[0].modes.len(), 2);
    }

    #[test]
    fn coherent_revisions_cover_multi_output_disabled_disconnect_and_mode_change() {
        let mut accumulator = DiscoveryAccumulator::new(3);
        for (head, name) in [(1, "HDMI-A-1"), (2, "DP-1")] {
            accumulator.introduce_head(head);
            accumulator.set_head_name(head, name.to_string());
            add_mode(&mut accumulator, head, head * 10, 1920, 1080, 60_000, true);
            accumulator.set_current_mode(head, head * 10);
        }
        accumulator.set_enabled(1, true);
        accumulator.set_enabled(2, false);
        accumulator.commit(1);
        assert_eq!(accumulator.state().revision, 1);
        assert!(!accumulator.state().active.heads[1].enabled);

        accumulator.finish_head(2);
        accumulator.commit(2);
        assert_eq!(accumulator.state().revision, 2);
        assert!(!accumulator.state().active.heads[1].connected);

        accumulator.set_mode_refresh(10, 59_940);
        assert_eq!(accumulator.state().revision, 2);
        accumulator.commit(3);
        assert_eq!(accumulator.state().revision, 3);
        accumulator.commit(3);
        assert_eq!(accumulator.state().revision, 3);
    }

    #[test]
    fn validation_rejects_stale_overlapping_and_zero_output_topologies() {
        let mut accumulator = DiscoveryAccumulator::new(4);
        for (head, name) in [(1, "HDMI-A-1"), (2, "DP-1")] {
            accumulator.introduce_head(head);
            accumulator.set_head_name(head, name.to_string());
            accumulator.set_enabled(head, true);
            add_mode(&mut accumulator, head, head * 10, 1920, 1080, 60_000, true);
            accumulator.set_current_mode(head, head * 10);
        }
        accumulator.commit(7);
        let state = accumulator.state();
        let overlapping = DisplayValidateRequest {
            contract_version: DISPLAYS_CONTRACT_VERSION,
            base_revision: state.revision,
            draft: state.active.clone(),
        };
        assert!(validate_topology(&state, &overlapping)
            .issues
            .iter()
            .any(|issue| issue.code == "overlapping_outputs"));

        let mut zero = overlapping.clone();
        for head in &mut zero.draft.heads {
            head.enabled = false;
        }
        assert!(validate_topology(&state, &zero)
            .issues
            .iter()
            .any(|issue| issue.code == "no_usable_output"));

        let mut stale = overlapping;
        stale.base_revision -= 1;
        assert!(validate_topology(&state, &stale)
            .issues
            .iter()
            .any(|issue| issue.code == "stale_revision"));
    }

    #[test]
    fn manager_finish_is_typed_unavailable() {
        let mut accumulator = DiscoveryAccumulator::new(4);
        accumulator.introduce_head(1);
        accumulator.set_head_name(1, "DP-1".to_string());
        accumulator.commit(1);
        accumulator.finish_manager();
        let state = accumulator.state();
        assert_eq!(
            state.availability.state,
            SettingsAvailabilityState::Unavailable
        );
        assert!(!state.capabilities.output_management);
        assert!(state.active.heads.is_empty());
    }

    fn fixture_head(id: &str, serial: &str, enabled: bool) -> DisplayHeadConfiguration {
        DisplayHeadConfiguration {
            id: id.to_string(),
            identity: DisplayHeadIdentity {
                name: id.to_string(),
                description: format!("Fixture {id}"),
                make: "Agora".to_string(),
                model: "Panel".to_string(),
                serial_number: serial.to_string(),
                physical_width_mm: 600,
                physical_height_mm: 340,
            },
            connected: true,
            enabled,
            modes: vec![DisplayMode {
                id: "preferred".to_string(),
                width: 1920,
                height: 1080,
                refresh_millihz: 60_000,
                preferred: true,
            }],
            current_mode_id: enabled.then(|| "preferred".to_string()),
            x: 0,
            y: 0,
            scale_milli: 1000,
            transform: DisplayTransform::Normal,
            adaptive_sync: false,
        }
    }

    #[test]
    fn profile_reconciliation_handles_docking_replacement_and_missing_modes() {
        let mut saved = fixture_head("DP-1", "stable-1", true);
        saved.x = 1920;
        saved.scale_milli = 1250;
        saved.modes[0].refresh_millihz = 59_951;
        let profile = DisplayTopology {
            serial: 1,
            heads: vec![saved, fixture_head("DP-2", "docked", true)],
        };
        let current = DisplayTopology {
            serial: 8,
            heads: vec![
                fixture_head("DP-9", "stable-1", true),
                fixture_head("HDMI-A-1", "replacement", true),
            ],
        };
        let (target, status) = reconcile_profile(&profile, &current);
        assert_eq!(status.state, DisplayReconciliationState::PartiallyMatched);
        assert_eq!(status.matched_heads, ["DP-9"]);
        assert_eq!(status.unmatched_profile_heads, ["DP-2"]);
        assert_eq!(target.heads[0].x, 1920);
        assert_eq!(target.heads[0].scale_milli, 1250);
        assert_eq!(
            target.heads[0].current_mode_id.as_deref(),
            Some("preferred")
        );
        assert_eq!(target.heads[1], current.heads[1]);
    }

    #[test]
    fn ambiguous_identity_is_not_projected_and_safe_fallback_keeps_one_output() {
        let saved = fixture_head("DP-1", "ambiguous", false);
        let profile = DisplayTopology {
            serial: 1,
            heads: vec![saved],
        };
        let first = fixture_head("DP-1", "ambiguous", false);
        let mut second = first.clone();
        second.id = "DP-2".to_string();
        let current = DisplayTopology {
            serial: 2,
            heads: vec![first, second],
        };
        let (target, status) = reconcile_profile(&profile, &current);
        assert_eq!(status.state, DisplayReconciliationState::SafeFallback);
        assert_eq!(status.unmatched_profile_heads, ["DP-1"]);
        assert!(target.heads.iter().any(|head| head.enabled));
    }
}
