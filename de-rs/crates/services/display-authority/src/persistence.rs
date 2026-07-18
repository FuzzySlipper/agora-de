use protocol_settings::{DisplayLeaseState, DisplayTopology};
use serde::{Deserialize, Serialize};
use std::fs::{self, OpenOptions};
use std::io::{self, Write};
use std::os::unix::fs::{OpenOptionsExt, PermissionsExt};
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

pub const DISPLAY_PROFILE_SCHEMA_VERSION: u16 = 1;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DisplayProfile {
    pub schema_version: u16,
    pub confirmed_at_unix_millis: u64,
    pub topology: DisplayTopology,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct StoredTransaction {
    pub schema_version: u16,
    pub transaction_id: String,
    pub deadline_unix_millis: u64,
    pub state: DisplayLeaseState,
    pub action: String,
    pub previous: DisplayTopology,
    pub proposed: DisplayTopology,
    pub detail: String,
}

pub fn load_profile(state_dir: &Path) -> Result<Option<DisplayProfile>, String> {
    load_json(&profile_path(state_dir)).and_then(|profile: Option<DisplayProfile>| {
        if let Some(profile) = &profile {
            if profile.schema_version != DISPLAY_PROFILE_SCHEMA_VERSION {
                return Err(format!(
                    "unsupported display profile schema version {}",
                    profile.schema_version
                ));
            }
        }
        Ok(profile)
    })
}

pub fn persist_profile(state_dir: &Path, topology: &DisplayTopology) -> Result<(), String> {
    let profile = DisplayProfile {
        schema_version: DISPLAY_PROFILE_SCHEMA_VERSION,
        confirmed_at_unix_millis: unix_millis(),
        topology: topology.clone(),
    };
    atomic_json(&profile_path(state_dir), &profile)
}

pub fn load_transaction(state_dir: &Path) -> Result<Option<StoredTransaction>, String> {
    load_json(&transaction_path(state_dir)).and_then(|transaction: Option<StoredTransaction>| {
        if let Some(transaction) = &transaction {
            if transaction.schema_version != DISPLAY_PROFILE_SCHEMA_VERSION {
                return Err(format!(
                    "unsupported display transaction schema version {}",
                    transaction.schema_version
                ));
            }
        }
        Ok(transaction)
    })
}

pub fn persist_transaction(
    state_dir: &Path,
    transaction: &StoredTransaction,
) -> Result<(), String> {
    atomic_json(&transaction_path(state_dir), transaction)
}

pub fn transaction_lock_path(state_dir: &Path) -> PathBuf {
    state_dir.join("display-transaction.lock")
}

pub fn acquire_transaction_lock(state_dir: &Path, transaction_id: &str) -> Result<(), String> {
    fs::create_dir_all(state_dir).map_err(|error| error.to_string())?;
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(transaction_lock_path(state_dir))
        .map_err(|error| {
            if error.kind() == io::ErrorKind::AlreadyExists {
                "another display transaction is active".to_string()
            } else {
                error.to_string()
            }
        })?;
    file.write_all(transaction_id.as_bytes())
        .map_err(|error| error.to_string())?;
    file.sync_all().map_err(|error| error.to_string())
}

pub fn release_transaction_lock(state_dir: &Path) {
    let _ = fs::remove_file(transaction_lock_path(state_dir));
}

pub fn unix_millis() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis()
        .try_into()
        .unwrap_or(u64::MAX)
}

fn profile_path(state_dir: &Path) -> PathBuf {
    state_dir.join("display-profile-v1.json")
}

fn transaction_path(state_dir: &Path) -> PathBuf {
    state_dir.join("display-transaction-v1.json")
}

fn load_json<T: for<'de> Deserialize<'de>>(path: &Path) -> Result<Option<T>, String> {
    match fs::read(path) {
        Ok(payload) => serde_json::from_slice(&payload)
            .map(Some)
            .map_err(|error| format!("decode {}: {error}", path.display())),
        Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(error) => Err(format!("read {}: {error}", path.display())),
    }
}

fn atomic_json<T: Serialize>(path: &Path, value: &T) -> Result<(), String> {
    let parent = path
        .parent()
        .ok_or_else(|| "display state path has no parent".to_string())?;
    fs::create_dir_all(parent).map_err(|error| format!("create {}: {error}", parent.display()))?;
    let payload = serde_json::to_vec_pretty(value).map_err(|error| error.to_string())?;
    let temporary = parent.join(format!(
        ".{}.{}.tmp",
        path.file_name()
            .and_then(|name| name.to_str())
            .unwrap_or("display-state"),
        std::process::id()
    ));
    let mut file = OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .mode(0o600)
        .open(&temporary)
        .map_err(|error| format!("create {}: {error}", temporary.display()))?;
    file.write_all(&payload)
        .and_then(|_| file.write_all(b"\n"))
        .and_then(|_| file.sync_all())
        .map_err(|error| format!("write {}: {error}", temporary.display()))?;
    fs::set_permissions(&temporary, fs::Permissions::from_mode(0o600))
        .map_err(|error| error.to_string())?;
    fs::rename(&temporary, path).map_err(|error| format!("replace {}: {error}", path.display()))?;
    FileSync::sync(parent)
}

struct FileSync;

impl FileSync {
    fn sync(path: &Path) -> Result<(), String> {
        let directory = fs::File::open(path).map_err(|error| error.to_string())?;
        directory.sync_all().map_err(|error| error.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use protocol_settings::DisplayTopology;

    fn temp_dir(name: &str) -> PathBuf {
        let path = std::env::temp_dir().join(format!(
            "agora-display-authority-{name}-{}-{}",
            std::process::id(),
            unix_millis()
        ));
        fs::create_dir_all(&path).expect("create temp dir");
        path
    }

    #[test]
    fn profile_persistence_is_atomic_and_strict() {
        let dir = temp_dir("profile");
        let topology = DisplayTopology {
            serial: 7,
            heads: Vec::new(),
        };
        persist_profile(&dir, &topology).expect("persist profile");
        let loaded = load_profile(&dir).expect("load profile").expect("profile");
        assert_eq!(loaded.schema_version, DISPLAY_PROFILE_SCHEMA_VERSION);
        assert_eq!(loaded.topology, topology);
        assert_eq!(
            fs::metadata(profile_path(&dir))
                .expect("metadata")
                .permissions()
                .mode()
                & 0o777,
            0o600
        );
        assert!(!fs::read_dir(&dir).expect("read dir").any(|entry| entry
            .expect("entry")
            .file_name()
            .to_string_lossy()
            .ends_with(".tmp")));
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn corrupt_truncated_and_future_profiles_fail_without_rewrite() {
        let dir = temp_dir("corrupt");
        fs::write(profile_path(&dir), b"{\"schemaVersion\":1").expect("write truncated profile");
        assert!(load_profile(&dir)
            .expect_err("truncated profile must fail")
            .contains("decode"));
        fs::write(
            profile_path(&dir),
            b"{\"schemaVersion\":99,\"confirmedAtUnixMillis\":0,\"topology\":{\"serial\":0,\"heads\":[]}}",
        )
        .expect("write future profile");
        assert!(load_profile(&dir)
            .expect_err("future profile must fail")
            .contains("unsupported display profile"));
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn transaction_lock_serializes_mutations() {
        let dir = temp_dir("lock");
        acquire_transaction_lock(&dir, "tx-1").expect("first lock");
        assert!(acquire_transaction_lock(&dir, "tx-2")
            .expect_err("overlap must fail")
            .contains("another display transaction"));
        release_transaction_lock(&dir);
        acquire_transaction_lock(&dir, "tx-3").expect("lock after release");
        release_transaction_lock(&dir);
        let _ = fs::remove_dir_all(dir);
    }
}
