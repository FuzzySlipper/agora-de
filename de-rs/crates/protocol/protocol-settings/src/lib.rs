use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet};

pub const SETTINGS_SCHEMA_VERSION: u16 = 1;
pub const DIAGNOSTICS_CONTRACT_VERSION: u16 = 1;
pub const DIAGNOSTICS_MODULE_ID: &str = "diagnostics";
pub const DISPLAYS_CONTRACT_VERSION: u16 = 1;
pub const DISPLAYS_MODULE_ID: &str = "displays";
pub const WINDOW_MANAGEMENT_CONTRACT_VERSION: u16 = 1;
pub const WINDOW_MANAGEMENT_MODULE_ID: &str = "window-management";
pub const APPEARANCE_CONTRACT_VERSION: u16 = 1;
pub const APPEARANCE_MODULE_ID: &str = "appearance";
pub const SHORTCUTS_CONTRACT_VERSION: u16 = 1;
pub const SHORTCUTS_MODULE_ID: &str = "shortcuts";

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SettingsCategory {
    Hardware,
    Personal,
    System,
}

impl SettingsCategory {
    pub const ALL: [Self; 3] = [Self::Hardware, Self::Personal, Self::System];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Hardware => "hardware",
            Self::Personal => "personal",
            Self::System => "system",
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SettingsCapability {
    Load,
    Validate,
    Apply,
    RestoreDefaults,
}

impl SettingsCapability {
    pub const ALL: [Self; 4] = [
        Self::Load,
        Self::Validate,
        Self::Apply,
        Self::RestoreDefaults,
    ];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Load => "load",
            Self::Validate => "validate",
            Self::Apply => "apply",
            Self::RestoreDefaults => "restore_defaults",
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SettingsAvailabilityState {
    Available,
    ReadOnly,
    Unavailable,
    Unsupported,
}

impl SettingsAvailabilityState {
    pub const ALL: [Self; 4] = [
        Self::Available,
        Self::ReadOnly,
        Self::Unavailable,
        Self::Unsupported,
    ];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Available => "available",
            Self::ReadOnly => "read_only",
            Self::Unavailable => "unavailable",
            Self::Unsupported => "unsupported",
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SettingsOperation {
    Load,
    Validate,
    Apply,
    Reset,
    RestoreDefaults,
    Keep,
    Revert,
}

impl SettingsOperation {
    pub const ALL: [Self; 7] = [
        Self::Load,
        Self::Validate,
        Self::Apply,
        Self::Reset,
        Self::RestoreDefaults,
        Self::Keep,
        Self::Revert,
    ];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Load => "load",
            Self::Validate => "validate",
            Self::Apply => "apply",
            Self::Reset => "reset",
            Self::RestoreDefaults => "restore_defaults",
            Self::Keep => "keep",
            Self::Revert => "revert",
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SettingsErrorCode {
    InvalidRequest,
    ValidationFailed,
    StaleRevision,
    Unsupported,
    Unavailable,
    Timeout,
    ApplyFailed,
    RollbackFailed,
    RestartRequired,
    TestFailed,
    CompositorCancelled,
    TransactionBusy,
    ConfirmationExpired,
}

impl SettingsErrorCode {
    pub const ALL: [Self; 13] = [
        Self::InvalidRequest,
        Self::ValidationFailed,
        Self::StaleRevision,
        Self::Unsupported,
        Self::Unavailable,
        Self::Timeout,
        Self::ApplyFailed,
        Self::RollbackFailed,
        Self::RestartRequired,
        Self::TestFailed,
        Self::CompositorCancelled,
        Self::TransactionBusy,
        Self::ConfirmationExpired,
    ];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::InvalidRequest => "invalid_request",
            Self::ValidationFailed => "validation_failed",
            Self::StaleRevision => "stale_revision",
            Self::Unsupported => "unsupported",
            Self::Unavailable => "unavailable",
            Self::Timeout => "timeout",
            Self::ApplyFailed => "apply_failed",
            Self::RollbackFailed => "rollback_failed",
            Self::RestartRequired => "restart_required",
            Self::TestFailed => "test_failed",
            Self::CompositorCancelled => "compositor_cancelled",
            Self::TransactionBusy => "transaction_busy",
            Self::ConfirmationExpired => "confirmation_expired",
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SettingsApplyOutcomeKind {
    Applied,
    RestartRequired,
    RolledBack,
    PendingConfirmation,
    Kept,
}

impl SettingsApplyOutcomeKind {
    pub const ALL: [Self; 5] = [
        Self::Applied,
        Self::RestartRequired,
        Self::RolledBack,
        Self::PendingConfirmation,
        Self::Kept,
    ];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Applied => "applied",
            Self::RestartRequired => "restart_required",
            Self::RolledBack => "rolled_back",
            Self::PendingConfirmation => "pending_confirmation",
            Self::Kept => "kept",
        }
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsModuleManifest {
    pub id: String,
    pub category: SettingsCategory,
    pub title: String,
    pub summary: String,
    pub icon: String,
    pub route: String,
    pub search_terms: Vec<String>,
    pub capabilities: Vec<SettingsCapability>,
    pub backend_adapter: String,
    pub ui_entry_point: String,
    pub contract_version: u16,
}

impl SettingsModuleManifest {
    pub fn validate(&self) -> Result<(), &'static str> {
        if !valid_stable_id(&self.id) {
            return Err("module id must be a stable lowercase identifier");
        }
        if !valid_stable_id(&self.route) {
            return Err("module route must be a stable lowercase identifier");
        }
        if self.title.trim().is_empty() || self.summary.trim().is_empty() {
            return Err("module title and summary are required");
        }
        if !valid_token(&self.icon)
            || !valid_token(&self.backend_adapter)
            || !valid_token(&self.ui_entry_point)
        {
            return Err("module icon and entry points must be stable tokens");
        }
        if self.contract_version == 0 {
            return Err("module contract version must be non-zero");
        }
        if self.capabilities.is_empty() || !self.capabilities.contains(&SettingsCapability::Load) {
            return Err("module must support load");
        }
        if self.search_terms.len() > 24
            || self
                .search_terms
                .iter()
                .any(|term| term.trim().is_empty() || term.len() > 64)
        {
            return Err("module search terms must be non-empty and bounded");
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsModuleAvailability {
    pub state: SettingsAvailabilityState,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsCatalogEntry {
    pub manifest: SettingsModuleManifest,
    pub availability: SettingsModuleAvailability,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsCatalogResponse {
    pub schema_version: u16,
    pub modules: Vec<SettingsCatalogEntry>,
}

impl SettingsCatalogResponse {
    pub fn validate(&self) -> Result<(), &'static str> {
        if self.schema_version != SETTINGS_SCHEMA_VERSION {
            return Err("unsupported settings catalog schema version");
        }
        let mut ids = BTreeSet::new();
        for module in &self.modules {
            module.manifest.validate()?;
            if !ids.insert(module.manifest.id.as_str()) {
                return Err("duplicate settings module id");
            }
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsValidationIssue {
    pub field: String,
    pub code: String,
    pub message: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsError {
    pub code: SettingsErrorCode,
    pub message: String,
    pub retryable: bool,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub issues: Vec<SettingsValidationIssue>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub restart_component: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsLoadRequest {
    pub contract_version: u16,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsResetRequest {
    pub contract_version: u16,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsDefaultsRequest {
    pub contract_version: u16,
    pub base_revision: u64,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DiagnosticsSettings {
    pub diagnostic_overlay_enabled: bool,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DiagnosticsServiceState {
    pub enabled: bool,
    pub active: bool,
    pub enabled_state: String,
    pub active_state: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DiagnosticsComponentHealth {
    pub id: String,
    pub label: String,
    pub state: String,
    pub version: String,
    pub detail: String,
    pub recovery: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DiagnosticsSupportBundle {
    pub schema_version: u16,
    pub generated_at_unix_millis: u64,
    pub product_version: String,
    pub settings_schema_version: u16,
    pub components: Vec<DiagnosticsComponentHealth>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DiagnosticsSettingsState {
    pub module_id: String,
    pub contract_version: u16,
    pub revision: u64,
    pub active: DiagnosticsSettings,
    pub defaults: DiagnosticsSettings,
    pub service: DiagnosticsServiceState,
    pub product_version: String,
    pub settings_schema_version: u16,
    pub components: Vec<DiagnosticsComponentHealth>,
    pub support_bundle: DiagnosticsSupportBundle,
    pub availability: SettingsModuleAvailability,
}

impl DiagnosticsSettingsState {
    pub fn validate(&self) -> Result<(), &'static str> {
        if self.module_id != DIAGNOSTICS_MODULE_ID {
            return Err("diagnostics state has the wrong module id");
        }
        if self.contract_version != DIAGNOSTICS_CONTRACT_VERSION {
            return Err("unsupported diagnostics contract version");
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DiagnosticsValidateRequest {
    pub contract_version: u16,
    pub base_revision: u64,
    pub draft: DiagnosticsSettings,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsValidationResponse {
    pub valid: bool,
    pub issues: Vec<SettingsValidationIssue>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DiagnosticsApplyRequest {
    pub contract_version: u16,
    pub base_revision: u64,
    pub draft: DiagnosticsSettings,
}

impl DiagnosticsApplyRequest {
    pub fn validate(&self) -> Result<(), &'static str> {
        if self.contract_version != DIAGNOSTICS_CONTRACT_VERSION {
            return Err("unsupported diagnostics contract version");
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SettingsApplyOutcome {
    pub kind: SettingsApplyOutcomeKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub restart_component: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DiagnosticsApplyResponse {
    pub state: DiagnosticsSettingsState,
    pub outcome: SettingsApplyOutcome,
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum WindowLayoutMode {
    Freeform,
    Zones,
    Columns,
}

impl WindowLayoutMode {
    pub const ALL: [Self; 3] = [Self::Freeform, Self::Zones, Self::Columns];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Freeform => "freeform",
            Self::Zones => "zones",
            Self::Columns => "columns",
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum WindowLayoutRule {
    Zones,
    MasterStack,
    Dwindle,
}

impl WindowLayoutRule {
    pub const ALL: [Self; 3] = [Self::Zones, Self::MasterStack, Self::Dwindle];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Zones => "zones",
            Self::MasterStack => "master_stack",
            Self::Dwindle => "dwindle",
        }
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct WindowManagementGaps {
    pub outer_horizontal: u16,
    pub outer_vertical: u16,
    pub inner_horizontal: u16,
    pub inner_vertical: u16,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct WindowManagementSettings {
    pub mode: WindowLayoutMode,
    pub rule: WindowLayoutRule,
    pub gaps: WindowManagementGaps,
    pub master_count: u8,
    pub master_ratio: f64,
    pub smart_gaps: bool,
}

impl WindowManagementSettings {
    pub fn validate(&self) -> Vec<SettingsValidationIssue> {
        let mut issues = Vec::new();
        if !(1..=8).contains(&self.master_count) {
            issues.push(SettingsValidationIssue {
                field: "masterCount".to_string(),
                code: "out_of_range".to_string(),
                message: "Master count must be between 1 and 8.".to_string(),
            });
        }
        if !(0.1..=0.9).contains(&self.master_ratio) || !self.master_ratio.is_finite() {
            issues.push(SettingsValidationIssue {
                field: "masterRatio".to_string(),
                code: "out_of_range".to_string(),
                message: "Master ratio must be between 10% and 90%.".to_string(),
            });
        }
        for (field, value) in [
            ("gaps.outerHorizontal", self.gaps.outer_horizontal),
            ("gaps.outerVertical", self.gaps.outer_vertical),
            ("gaps.innerHorizontal", self.gaps.inner_horizontal),
            ("gaps.innerVertical", self.gaps.inner_vertical),
        ] {
            if value > 128 {
                issues.push(SettingsValidationIssue {
                    field: field.to_string(),
                    code: "out_of_range".to_string(),
                    message: "Gaps must be between 0 and 128 pixels.".to_string(),
                });
            }
        }
        issues
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct WindowWorkspaceSummary {
    pub id: String,
    pub name: String,
    pub output_id: String,
    pub active: bool,
    pub surface_count: u32,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct WindowManagementSettingsState {
    pub module_id: String,
    pub contract_version: u16,
    pub revision: u64,
    pub active: WindowManagementSettings,
    pub defaults: WindowManagementSettings,
    pub workspaces: Vec<WindowWorkspaceSummary>,
    pub availability: SettingsModuleAvailability,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct WindowManagementValidateRequest {
    pub contract_version: u16,
    pub base_revision: u64,
    pub draft: WindowManagementSettings,
}

pub type WindowManagementApplyRequest = WindowManagementValidateRequest;

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct WindowManagementApplyResponse {
    pub state: WindowManagementSettingsState,
    pub outcome: SettingsApplyOutcome,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct AppearanceThemeSummary {
    pub id: String,
    pub name: String,
    pub tokens: BTreeMap<String, String>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct AppearanceSettings {
    pub theme_id: String,
}

impl AppearanceSettings {
    pub fn validate(&self) -> Result<(), &'static str> {
        if valid_stable_id(&self.theme_id) { Ok(()) } else { Err("invalid theme id") }
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct AppearanceSettingsState {
    pub module_id: String,
    pub contract_version: u16,
    pub revision: u64,
    pub active: AppearanceSettings,
    pub defaults: AppearanceSettings,
    pub themes: Vec<AppearanceThemeSummary>,
    pub restart_required: bool,
    pub availability: SettingsModuleAvailability,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct AppearanceValidateRequest {
    pub contract_version: u16,
    pub base_revision: u64,
    pub draft: AppearanceSettings,
}

pub type AppearanceApplyRequest = AppearanceValidateRequest;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct AppearanceApplyResponse {
    pub state: AppearanceSettingsState,
    pub outcome: SettingsApplyOutcome,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ShortcutDefinition {
    pub id: String,
    pub title: String,
    pub group: String,
    pub reserved: bool,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ShortcutAssignment {
    pub id: String,
    pub accelerator: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ShortcutKeymap {
    pub assignments: Vec<ShortcutAssignment>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ShortcutSettingsState {
    pub module_id: String,
    pub contract_version: u16,
    pub revision: u64,
    pub active: ShortcutKeymap,
    pub defaults: ShortcutKeymap,
    pub definitions: Vec<ShortcutDefinition>,
    pub availability: SettingsModuleAvailability,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ShortcutValidateRequest {
    pub contract_version: u16,
    pub base_revision: u64,
    pub draft: ShortcutKeymap,
}

pub type ShortcutApplyRequest = ShortcutValidateRequest;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ShortcutApplyResponse {
    pub state: ShortcutSettingsState,
    pub outcome: SettingsApplyOutcome,
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum DisplayTransform {
    Normal,
    #[serde(rename = "rotate_90")]
    Rotate90,
    #[serde(rename = "rotate_180")]
    Rotate180,
    #[serde(rename = "rotate_270")]
    Rotate270,
    Flipped,
    #[serde(rename = "flipped_90")]
    Flipped90,
    #[serde(rename = "flipped_180")]
    Flipped180,
    #[serde(rename = "flipped_270")]
    Flipped270,
}

impl DisplayTransform {
    pub const ALL: [Self; 8] = [
        Self::Normal,
        Self::Rotate90,
        Self::Rotate180,
        Self::Rotate270,
        Self::Flipped,
        Self::Flipped90,
        Self::Flipped180,
        Self::Flipped270,
    ];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Normal => "normal",
            Self::Rotate90 => "rotate_90",
            Self::Rotate180 => "rotate_180",
            Self::Rotate270 => "rotate_270",
            Self::Flipped => "flipped",
            Self::Flipped90 => "flipped_90",
            Self::Flipped180 => "flipped_180",
            Self::Flipped270 => "flipped_270",
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum DisplayLeaseState {
    Pending,
    Kept,
    Reverted,
    TimedOut,
    RollbackFailed,
}

impl DisplayLeaseState {
    pub const ALL: [Self; 5] = [
        Self::Pending,
        Self::Kept,
        Self::Reverted,
        Self::TimedOut,
        Self::RollbackFailed,
    ];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::Pending => "pending",
            Self::Kept => "kept",
            Self::Reverted => "reverted",
            Self::TimedOut => "timed_out",
            Self::RollbackFailed => "rollback_failed",
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum DisplayReconciliationState {
    NotNeeded,
    Applied,
    PartiallyMatched,
    SafeFallback,
    Failed,
}

impl DisplayReconciliationState {
    pub const ALL: [Self; 5] = [
        Self::NotNeeded,
        Self::Applied,
        Self::PartiallyMatched,
        Self::SafeFallback,
        Self::Failed,
    ];

    pub const fn wire_name(self) -> &'static str {
        match self {
            Self::NotNeeded => "not_needed",
            Self::Applied => "applied",
            Self::PartiallyMatched => "partially_matched",
            Self::SafeFallback => "safe_fallback",
            Self::Failed => "failed",
        }
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayProtocolCapabilities {
    pub output_management: bool,
    pub protocol_version: u32,
    pub test_configuration: bool,
    pub apply_configuration: bool,
    pub adaptive_sync: bool,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayMode {
    pub id: String,
    pub width: i32,
    pub height: i32,
    pub refresh_millihz: i32,
    pub preferred: bool,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayHeadIdentity {
    pub name: String,
    pub description: String,
    pub make: String,
    pub model: String,
    pub serial_number: String,
    pub physical_width_mm: i32,
    pub physical_height_mm: i32,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayHeadConfiguration {
    pub id: String,
    pub identity: DisplayHeadIdentity,
    pub connected: bool,
    pub enabled: bool,
    pub modes: Vec<DisplayMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub current_mode_id: Option<String>,
    pub x: i32,
    pub y: i32,
    pub scale_milli: u32,
    pub transform: DisplayTransform,
    pub adaptive_sync: bool,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayTopology {
    pub serial: u32,
    pub heads: Vec<DisplayHeadConfiguration>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayConfirmationLease {
    pub transaction_id: String,
    pub state: DisplayLeaseState,
    pub deadline_unix_millis: u64,
    pub remaining_millis: u64,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayReconciliationStatus {
    pub state: DisplayReconciliationState,
    pub detail: String,
    pub matched_heads: Vec<String>,
    pub unmatched_profile_heads: Vec<String>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplaySettingsState {
    pub module_id: String,
    pub contract_version: u16,
    pub revision: u64,
    pub active: DisplayTopology,
    pub defaults: DisplayTopology,
    pub capabilities: DisplayProtocolCapabilities,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub lease: Option<DisplayConfirmationLease>,
    pub reconciliation: DisplayReconciliationStatus,
    pub availability: SettingsModuleAvailability,
}

impl DisplaySettingsState {
    pub fn validate(&self) -> Result<(), &'static str> {
        if self.module_id != DISPLAYS_MODULE_ID {
            return Err("display state has the wrong module id");
        }
        if self.contract_version != DISPLAYS_CONTRACT_VERSION {
            return Err("unsupported displays contract version");
        }
        if self.availability.state == SettingsAvailabilityState::Available
            && !self.capabilities.output_management
        {
            return Err("available displays state requires output-management");
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayValidateRequest {
    pub contract_version: u16,
    pub base_revision: u64,
    pub draft: DisplayTopology,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayApplyRequest {
    pub contract_version: u16,
    pub base_revision: u64,
    pub draft: DisplayTopology,
    pub confirmation_timeout_millis: u64,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayApplyResponse {
    pub state: DisplaySettingsState,
    pub outcome: SettingsApplyOutcome,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayLeaseActionRequest {
    pub contract_version: u16,
    pub transaction_id: String,
}

fn valid_stable_id(value: &str) -> bool {
    !value.is_empty()
        && value.len() <= 64
        && value
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-')
}

fn valid_token(value: &str) -> bool {
    !value.is_empty()
        && value.len() <= 96
        && value.bytes().all(|byte| {
            byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.' | b'/' | b'@')
        })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn diagnostics_manifest() -> SettingsModuleManifest {
        SettingsModuleManifest {
            id: DIAGNOSTICS_MODULE_ID.to_string(),
            category: SettingsCategory::System,
            title: "Diagnostics & About".to_string(),
            summary: "Inspect Agora services and diagnostic tools.".to_string(),
            icon: "diagnostics".to_string(),
            route: DIAGNOSTICS_MODULE_ID.to_string(),
            search_terms: vec!["overlay".to_string(), "health".to_string()],
            capabilities: SettingsCapability::ALL.to_vec(),
            backend_adapter: "settings-diagnostics".to_string(),
            ui_entry_point: "settings-diagnostics".to_string(),
            contract_version: DIAGNOSTICS_CONTRACT_VERSION,
        }
    }

    #[test]
    fn wire_vocabularies_are_stable() {
        assert_eq!(
            SettingsErrorCode::ALL.map(SettingsErrorCode::wire_name),
            [
                "invalid_request",
                "validation_failed",
                "stale_revision",
                "unsupported",
                "unavailable",
                "timeout",
                "apply_failed",
                "rollback_failed",
                "restart_required",
                "test_failed",
                "compositor_cancelled",
                "transaction_busy",
                "confirmation_expired",
            ]
        );
        assert_eq!(
            SettingsOperation::ALL.map(SettingsOperation::wire_name),
            [
                "load",
                "validate",
                "apply",
                "reset",
                "restore_defaults",
                "keep",
                "revert",
            ]
        );
    }

    #[test]
    fn catalog_round_trips_and_unknown_modules_degrade_explicitly() {
        let catalog = SettingsCatalogResponse {
            schema_version: SETTINGS_SCHEMA_VERSION,
            modules: vec![
                SettingsCatalogEntry {
                    manifest: diagnostics_manifest(),
                    availability: SettingsModuleAvailability {
                        state: SettingsAvailabilityState::Available,
                        reason: None,
                    },
                },
                SettingsCatalogEntry {
                    manifest: SettingsModuleManifest {
                        id: "future-module".to_string(),
                        category: SettingsCategory::System,
                        title: "Future module".to_string(),
                        summary: "A newer module unavailable to this build.".to_string(),
                        icon: "settings".to_string(),
                        route: "future-module".to_string(),
                        search_terms: Vec::new(),
                        capabilities: vec![SettingsCapability::Load],
                        backend_adapter: "future-module".to_string(),
                        ui_entry_point: "future-module".to_string(),
                        contract_version: 99,
                    },
                    availability: SettingsModuleAvailability {
                        state: SettingsAvailabilityState::Unsupported,
                        reason: Some("contract version 99 is unsupported".to_string()),
                    },
                },
            ],
        };

        catalog.validate().expect("valid catalog");
        let json = serde_json::to_string(&catalog).expect("serialize catalog");
        let decoded: SettingsCatalogResponse =
            serde_json::from_str(&json).expect("deserialize catalog");
        assert_eq!(decoded, catalog);
        assert_eq!(
            decoded.modules[1].availability.state,
            SettingsAvailabilityState::Unsupported
        );
    }

    #[test]
    fn diagnostics_toggle_round_trips_without_a_generic_payload() {
        let request = DiagnosticsApplyRequest {
            contract_version: DIAGNOSTICS_CONTRACT_VERSION,
            base_revision: 7,
            draft: DiagnosticsSettings {
                diagnostic_overlay_enabled: true,
            },
        };
        let json = serde_json::to_string(&request).expect("serialize request");
        let decoded: DiagnosticsApplyRequest =
            serde_json::from_str(&json).expect("deserialize request");
        assert_eq!(decoded, request);
        decoded.validate().expect("supported contract");
        assert!(json.contains("diagnosticOverlayEnabled"));
        assert!(!json.contains("payload"));
    }

    #[test]
    fn removed_and_unknown_fields_fail_strict_decoding() {
        let missing = r#"{"contractVersion":1,"draft":{"diagnosticOverlayEnabled":true}}"#;
        assert!(serde_json::from_str::<DiagnosticsApplyRequest>(missing).is_err());

        let unknown = r#"{"contractVersion":1,"baseRevision":0,"draft":{"diagnosticOverlayEnabled":true,"command":"nope"}}"#;
        assert!(serde_json::from_str::<DiagnosticsApplyRequest>(unknown).is_err());
    }

    #[test]
    fn display_snapshot_round_trips_fractional_scale_and_transform() {
        let state = DisplaySettingsState {
            module_id: DISPLAYS_MODULE_ID.to_string(),
            contract_version: DISPLAYS_CONTRACT_VERSION,
            revision: 4,
            active: DisplayTopology {
                serial: 91,
                heads: vec![DisplayHeadConfiguration {
                    id: "HDMI-A-1".to_string(),
                    identity: DisplayHeadIdentity {
                        name: "HDMI-A-1".to_string(),
                        description: "Example display".to_string(),
                        make: "Agora".to_string(),
                        model: "Panel".to_string(),
                        serial_number: "serial-1".to_string(),
                        physical_width_mm: 600,
                        physical_height_mm: 340,
                    },
                    connected: true,
                    enabled: true,
                    modes: vec![DisplayMode {
                        id: "2560x1440@60000".to_string(),
                        width: 2560,
                        height: 1440,
                        refresh_millihz: 60_000,
                        preferred: true,
                    }],
                    current_mode_id: Some("2560x1440@60000".to_string()),
                    x: 2560,
                    y: 0,
                    scale_milli: 1250,
                    transform: DisplayTransform::Rotate90,
                    adaptive_sync: false,
                }],
            },
            defaults: DisplayTopology {
                serial: 91,
                heads: Vec::new(),
            },
            capabilities: DisplayProtocolCapabilities {
                output_management: true,
                protocol_version: 4,
                test_configuration: true,
                apply_configuration: true,
                adaptive_sync: true,
            },
            lease: None,
            reconciliation: DisplayReconciliationStatus {
                state: DisplayReconciliationState::NotNeeded,
                detail: "no stored profile".to_string(),
                matched_heads: Vec::new(),
                unmatched_profile_heads: Vec::new(),
            },
            availability: SettingsModuleAvailability {
                state: SettingsAvailabilityState::Available,
                reason: None,
            },
        };
        state.validate().expect("valid display state");
        let json = serde_json::to_string(&state).expect("serialize display state");
        let decoded: DisplaySettingsState =
            serde_json::from_str(&json).expect("deserialize display state");
        assert_eq!(decoded, state);
        assert!(json.contains("\"scaleMilli\":1250"));
        assert!(json.contains("\"transform\":\"rotate_90\""));
    }
}
