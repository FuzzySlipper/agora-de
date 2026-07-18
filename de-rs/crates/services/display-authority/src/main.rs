use display_authority::{
    acquire_transaction_lock, apply_from_environment, discover_from_environment, load_profile,
    load_transaction, persist_profile, persist_transaction, rebase_topology, reconcile_profile,
    release_transaction_lock, test_from_environment, unix_millis, ConfigurationError,
    StoredTransaction, DISPLAY_PROFILE_SCHEMA_VERSION,
};
use protocol_settings::{
    DisplayApplyRequest, DisplayApplyResponse, DisplayConfirmationLease, DisplayLeaseActionRequest,
    DisplayLeaseState, DisplayReconciliationState, DisplaySettingsState, DisplayValidateRequest,
    SettingsApplyOutcome, SettingsApplyOutcomeKind, SettingsError, SettingsErrorCode,
    DISPLAYS_CONTRACT_VERSION,
};
use serde::de::DeserializeOwned;
use serde::Serialize;
use std::io::{self, Read};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::thread;
use std::time::Duration;

fn main() {
    if let Err(error) = run() {
        emit(&error);
        std::process::exit(1);
    }
}

fn run() -> Result<(), SettingsError> {
    let mut arguments = std::env::args().skip(1);
    let command = arguments.next().unwrap_or_else(|| "snapshot".to_string());
    let state_dir = parse_state_dir(arguments.collect())?;
    match command.as_str() {
        "snapshot" => emit(&decorated_state(&state_dir)?),
        "validate" => {
            let request: DisplayValidateRequest = read_stdin()?;
            let current = discover_from_environment();
            let validation = display_authority::validate_topology(&current, &request);
            if validation.valid {
                test_from_environment(&request).map_err(configuration_error)?;
            }
            emit(&validation);
        }
        "apply" => apply(&state_dir)?,
        "keep" => lease_action(&state_dir, "keep")?,
        "revert" => lease_action(&state_dir, "revert")?,
        "reconcile" => reconcile(&state_dir)?,
        "watch" => watch(&state_dir)?,
        "lease-worker" => lease_worker(&state_dir)?,
        _ => return Err(invalid("unknown display authority command")),
    }
    Ok(())
}

fn parse_state_dir(arguments: Vec<String>) -> Result<PathBuf, SettingsError> {
    let mut state_dir = std::env::var_os("XDG_STATE_HOME")
        .map(PathBuf::from)
        .or_else(|| std::env::var_os("HOME").map(|home| PathBuf::from(home).join(".local/state")))
        .unwrap_or_else(|| PathBuf::from("/tmp"))
        .join("agora-de/displays");
    let mut index = 0;
    while index < arguments.len() {
        if arguments[index] != "--state-dir" || index + 1 >= arguments.len() {
            return Err(invalid("expected --state-dir PATH"));
        }
        state_dir = PathBuf::from(&arguments[index + 1]);
        index += 2;
    }
    Ok(state_dir)
}

fn decorated_state(state_dir: &Path) -> Result<DisplaySettingsState, SettingsError> {
    recover_expired_transaction(state_dir)?;
    let mut state = discover_from_environment();
    if let Some(transaction) = load_transaction(state_dir).map_err(storage_error)? {
        let remaining = transaction
            .deadline_unix_millis
            .saturating_sub(unix_millis());
        state.lease = Some(DisplayConfirmationLease {
            transaction_id: transaction.transaction_id,
            state: transaction.state,
            deadline_unix_millis: transaction.deadline_unix_millis,
            remaining_millis: remaining,
        });
    }
    match load_profile(state_dir) {
        Ok(Some(profile)) => {
            let (_, status) = reconcile_profile(&profile.topology, &state.active);
            state.reconciliation = status;
        }
        Ok(None) => {}
        Err(error) => {
            state.reconciliation.state = DisplayReconciliationState::Failed;
            state.reconciliation.detail = error;
        }
    }
    Ok(state)
}

fn apply(state_dir: &Path) -> Result<(), SettingsError> {
    let request: DisplayApplyRequest = read_stdin()?;
    if !(5_000..=60_000).contains(&request.confirmation_timeout_millis) {
        return Err(invalid(
            "confirmationTimeoutMillis must be between 5000 and 60000",
        ));
    }
    let transaction_id = format!("display-{}-{}", unix_millis(), std::process::id());
    acquire_transaction_lock(state_dir, &transaction_id).map_err(|error| SettingsError {
        code: SettingsErrorCode::TransactionBusy,
        message: error,
        retryable: true,
        issues: Vec::new(),
        restart_component: None,
    })?;
    let result = apply_locked(state_dir, &transaction_id, &request);
    if result.is_err() {
        release_transaction_lock(state_dir);
    }
    let response = result?;
    emit(&response);
    Ok(())
}

fn apply_locked(
    state_dir: &Path,
    transaction_id: &str,
    request: &DisplayApplyRequest,
) -> Result<DisplayApplyResponse, SettingsError> {
    let before = discover_from_environment();
    let previous = before.active.clone();
    let validate = DisplayValidateRequest {
        contract_version: request.contract_version,
        base_revision: request.base_revision,
        draft: request.draft.clone(),
    };
    test_from_environment(&validate).map_err(configuration_error)?;
    let mut state = apply_from_environment(request).map_err(configuration_error)?;
    let deadline = unix_millis().saturating_add(request.confirmation_timeout_millis);
    let transaction = StoredTransaction {
        schema_version: DISPLAY_PROFILE_SCHEMA_VERSION,
        transaction_id: transaction_id.to_string(),
        deadline_unix_millis: deadline,
        state: DisplayLeaseState::Pending,
        action: "pending".to_string(),
        previous,
        proposed: request.draft.clone(),
        detail: "waiting for display confirmation".to_string(),
    };
    if let Err(error) = persist_transaction(state_dir, &transaction) {
        rollback_after_confirmation_failure(&transaction.previous)?;
        return Err(storage_error(format!(
            "could not record display confirmation; previous configuration restored: {error}"
        )));
    }
    if let Err(error) = spawn_worker(state_dir) {
        rollback_after_confirmation_failure(&transaction.previous)?;
        return Err(error);
    }
    state.lease = Some(DisplayConfirmationLease {
        transaction_id: transaction_id.to_string(),
        state: DisplayLeaseState::Pending,
        deadline_unix_millis: deadline,
        remaining_millis: request.confirmation_timeout_millis,
    });
    Ok(DisplayApplyResponse {
        state,
        outcome: SettingsApplyOutcome {
            kind: SettingsApplyOutcomeKind::PendingConfirmation,
            restart_component: None,
        },
    })
}

fn rollback_after_confirmation_failure(
    previous: &protocol_settings::DisplayTopology,
) -> Result<(), SettingsError> {
    let current = discover_from_environment();
    let rollback = rebase_topology(previous, &current.active);
    apply_from_environment(&DisplayApplyRequest {
        contract_version: DISPLAYS_CONTRACT_VERSION,
        base_revision: current.revision,
        draft: rollback,
        confirmation_timeout_millis: 5_000,
    })
    .map(|_| ())
    .map_err(|error| SettingsError {
        code: SettingsErrorCode::RollbackFailed,
        message: format!("confirmation could not be recorded and rollback failed: {error:?}"),
        retryable: false,
        issues: Vec::new(),
        restart_component: None,
    })
}

fn lease_action(state_dir: &Path, action: &str) -> Result<(), SettingsError> {
    let request: DisplayLeaseActionRequest = read_stdin()?;
    if request.contract_version != DISPLAYS_CONTRACT_VERSION {
        return Err(invalid("unsupported displays contract version"));
    }
    let mut transaction = load_transaction(state_dir)
        .map_err(storage_error)?
        .ok_or_else(|| expired("display confirmation transaction no longer exists"))?;
    if transaction.transaction_id != request.transaction_id
        || transaction.state != DisplayLeaseState::Pending
    {
        return Err(expired(
            "display confirmation transaction is no longer pending",
        ));
    }
    transaction.action = action.to_string();
    persist_transaction(state_dir, &transaction).map_err(storage_error)?;
    let until = unix_millis().saturating_add(5_000);
    loop {
        let current = load_transaction(state_dir)
            .map_err(storage_error)?
            .ok_or_else(|| expired("display transaction disappeared"))?;
        if current.state != DisplayLeaseState::Pending {
            let mut state = decorated_state(state_dir)?;
            state.lease = Some(DisplayConfirmationLease {
                transaction_id: current.transaction_id,
                state: current.state,
                deadline_unix_millis: current.deadline_unix_millis,
                remaining_millis: 0,
            });
            let kind = match current.state {
                DisplayLeaseState::Kept => SettingsApplyOutcomeKind::Kept,
                _ => SettingsApplyOutcomeKind::RolledBack,
            };
            emit(&DisplayApplyResponse {
                state,
                outcome: SettingsApplyOutcome {
                    kind,
                    restart_component: None,
                },
            });
            return Ok(());
        }
        if unix_millis() >= until {
            return Err(SettingsError {
                code: SettingsErrorCode::Timeout,
                message: "display confirmation action timed out".to_string(),
                retryable: true,
                issues: Vec::new(),
                restart_component: None,
            });
        }
        thread::sleep(Duration::from_millis(50));
    }
}

fn spawn_worker(state_dir: &Path) -> Result<(), SettingsError> {
    Command::new(std::env::current_exe().map_err(|error| storage_error(error.to_string()))?)
        .arg("lease-worker")
        .arg("--state-dir")
        .arg(state_dir)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .map_err(|error| storage_error(format!("spawn confirmation worker: {error}")))?;
    Ok(())
}

fn lease_worker(state_dir: &Path) -> Result<(), SettingsError> {
    loop {
        let Some(mut transaction) = load_transaction(state_dir).map_err(storage_error)? else {
            release_transaction_lock(state_dir);
            return Ok(());
        };
        if transaction.state != DisplayLeaseState::Pending {
            release_transaction_lock(state_dir);
            return Ok(());
        }
        if transaction.action == "keep" {
            persist_profile(state_dir, &transaction.proposed).map_err(storage_error)?;
            transaction.state = DisplayLeaseState::Kept;
            transaction.detail = "display configuration confirmed".to_string();
            persist_transaction(state_dir, &transaction).map_err(storage_error)?;
            release_transaction_lock(state_dir);
            return Ok(());
        }
        let timed_out = unix_millis() >= transaction.deadline_unix_millis;
        if transaction.action == "revert" || timed_out {
            let current = discover_from_environment();
            let rollback = rebase_topology(&transaction.previous, &current.active);
            let request = DisplayApplyRequest {
                contract_version: DISPLAYS_CONTRACT_VERSION,
                base_revision: current.revision,
                draft: rollback,
                confirmation_timeout_millis: 5_000,
            };
            transaction.state = match apply_from_environment(&request) {
                Ok(_) if timed_out => DisplayLeaseState::TimedOut,
                Ok(_) => DisplayLeaseState::Reverted,
                Err(error) => {
                    transaction.detail = format!("display rollback failed: {error:?}");
                    DisplayLeaseState::RollbackFailed
                }
            };
            if transaction.state != DisplayLeaseState::RollbackFailed {
                transaction.detail = if timed_out {
                    "confirmation timed out; previous configuration restored"
                } else {
                    "previous configuration restored"
                }
                .to_string();
            }
            persist_transaction(state_dir, &transaction).map_err(storage_error)?;
            release_transaction_lock(state_dir);
            return Ok(());
        }
        thread::sleep(Duration::from_millis(100));
    }
}

fn recover_expired_transaction(state_dir: &Path) -> Result<(), SettingsError> {
    let Some(transaction) = load_transaction(state_dir).map_err(storage_error)? else {
        return Ok(());
    };
    if transaction.state == DisplayLeaseState::Pending
        && unix_millis() >= transaction.deadline_unix_millis
    {
        // A crashed foreground service cannot strand an unsafe configuration.
        lease_worker(state_dir)?;
    }
    Ok(())
}

fn reconcile(state_dir: &Path) -> Result<(), SettingsError> {
    reconcile_once(state_dir, true)
}

fn reconcile_once(state_dir: &Path, emit_state: bool) -> Result<(), SettingsError> {
    recover_expired_transaction(state_dir)?;
    if load_transaction(state_dir)
        .map_err(storage_error)?
        .is_some_and(|transaction| transaction.state == DisplayLeaseState::Pending)
    {
        return Ok(());
    }
    let Some(profile) = load_profile(state_dir).map_err(storage_error)? else {
        if emit_state {
            emit(&decorated_state(state_dir)?);
        }
        return Ok(());
    };
    let current = discover_from_environment();
    let (target, _) = reconcile_profile(&profile.topology, &current.active);
    if target != current.active {
        let validate = DisplayValidateRequest {
            contract_version: DISPLAYS_CONTRACT_VERSION,
            base_revision: current.revision,
            draft: target,
        };
        test_from_environment(&validate).map_err(configuration_error)?;
        apply_from_environment(&DisplayApplyRequest {
            contract_version: validate.contract_version,
            base_revision: validate.base_revision,
            draft: validate.draft,
            confirmation_timeout_millis: 5_000,
        })
        .map_err(configuration_error)?;
    }
    if emit_state {
        emit(&decorated_state(state_dir)?);
    }
    Ok(())
}

fn watch(state_dir: &Path) -> Result<(), SettingsError> {
    loop {
        if let Err(error) = reconcile_once(state_dir, false) {
            eprintln!("display reconciliation: {}", error.message);
        }
        thread::sleep(Duration::from_secs(2));
    }
}

fn read_stdin<T: DeserializeOwned>() -> Result<T, SettingsError> {
    let mut input = Vec::new();
    io::stdin()
        .read_to_end(&mut input)
        .map_err(|error| invalid(&error.to_string()))?;
    let mut deserializer = serde_json::Deserializer::from_slice(&input);
    T::deserialize(&mut deserializer).map_err(|error| invalid(&format!("decode request: {error}")))
}

fn emit<T: Serialize>(value: &T) {
    match serde_json::to_string(value) {
        Ok(json) => println!("{json}"),
        Err(error) => {
            eprintln!("encode display authority response: {error}");
            std::process::exit(1);
        }
    }
}

fn configuration_error(error: ConfigurationError) -> SettingsError {
    match error {
        ConfigurationError::Validation(issues) => SettingsError {
            code: SettingsErrorCode::ValidationFailed,
            message: "display configuration is invalid".to_string(),
            retryable: false,
            issues,
            restart_component: None,
        },
        ConfigurationError::StaleRevision => SettingsError {
            code: SettingsErrorCode::StaleRevision,
            message: "display state changed; reload before applying".to_string(),
            retryable: true,
            issues: Vec::new(),
            restart_component: None,
        },
        ConfigurationError::TestFailed => simple(
            SettingsErrorCode::TestFailed,
            "compositor rejected the display configuration test",
            true,
        ),
        ConfigurationError::ApplyFailed => simple(
            SettingsErrorCode::ApplyFailed,
            "compositor rejected the display configuration",
            true,
        ),
        ConfigurationError::Cancelled => simple(
            SettingsErrorCode::CompositorCancelled,
            "compositor cancelled the display configuration",
            true,
        ),
        ConfigurationError::Unavailable(message) => {
            simple(SettingsErrorCode::Unavailable, &message, true)
        }
    }
}

fn invalid(message: &str) -> SettingsError {
    simple(SettingsErrorCode::InvalidRequest, message, false)
}
fn expired(message: &str) -> SettingsError {
    simple(SettingsErrorCode::ConfirmationExpired, message, false)
}
fn storage_error(message: String) -> SettingsError {
    simple(SettingsErrorCode::Unavailable, &message, true)
}
fn simple(code: SettingsErrorCode, message: &str, retryable: bool) -> SettingsError {
    SettingsError {
        code,
        message: message.to_string(),
        retryable,
        issues: Vec::new(),
        restart_component: None,
    }
}
