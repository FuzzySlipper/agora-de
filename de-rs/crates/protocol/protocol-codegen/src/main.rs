use protocol_codegen::{generate_go_settings_contracts, generate_typescript_contracts};
use std::env;
use std::fs;
use std::path::PathBuf;

fn main() {
    let mut args = env::args().skip(1);
    let mode = args.next().unwrap_or_else(|| "--print".to_string());
    match mode.as_str() {
        "--print" => {
            print!("{}", generate_typescript_contracts());
        }
        "--write" => {
            let path = required_path(args.next(), "--write");
            fs::write(&path, generate_typescript_contracts()).unwrap_or_else(|error| {
                panic!("failed to write {}: {error}", path.display());
            });
        }
        "--check" => {
            let path = required_path(args.next(), "--check");
            check_generated(path, generate_typescript_contracts(), "protocol contracts");
        }
        "--write-go-settings" => {
            let path = required_path(args.next(), "--write-go-settings");
            fs::write(&path, generate_go_settings_contracts()).unwrap_or_else(|error| {
                panic!("failed to write {}: {error}", path.display());
            });
        }
        "--check-go-settings" => {
            let path = required_path(args.next(), "--check-go-settings");
            check_generated(
                path,
                generate_go_settings_contracts(),
                "Go settings contracts",
            );
        }
        other => {
            eprintln!("unknown protocol-codegen mode: {other}");
            eprintln!("usage: protocol-codegen [--print|--write PATH|--check PATH|--write-go-settings PATH|--check-go-settings PATH]");
            std::process::exit(2);
        }
    }
}

fn check_generated(path: PathBuf, generated: String, label: &str) {
    let current = fs::read_to_string(&path).unwrap_or_else(|error| {
        panic!("failed to read {}: {error}", path.display());
    });
    if current != generated {
        eprintln!("generated {label} are out of date: {}", path.display());
        std::process::exit(1);
    }
    println!("{label}: OK");
}

fn required_path(value: Option<String>, mode: &str) -> PathBuf {
    value
        .map(PathBuf::from)
        .unwrap_or_else(|| panic!("{mode} requires a path"))
}
