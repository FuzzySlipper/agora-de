use protocol_codegen::generate_typescript_contracts;
use std::env;
use std::fs;
use std::path::PathBuf;

fn main() {
    let mut args = env::args().skip(1);
    let mode = args.next().unwrap_or_else(|| "--print".to_string());
    let generated = generate_typescript_contracts();

    match mode.as_str() {
        "--print" => {
            print!("{generated}");
        }
        "--write" => {
            let path = required_path(args.next(), "--write");
            fs::write(&path, generated).unwrap_or_else(|error| {
                panic!("failed to write {}: {error}", path.display());
            });
        }
        "--check" => {
            let path = required_path(args.next(), "--check");
            let current = fs::read_to_string(&path).unwrap_or_else(|error| {
                panic!("failed to read {}: {error}", path.display());
            });
            if current != generated {
                eprintln!("generated protocol contracts are out of date: {}", path.display());
                eprintln!("run: cargo run -p protocol-codegen -- --write {}", path.display());
                std::process::exit(1);
            }
            println!("protocol contracts: OK");
        }
        other => {
            eprintln!("unknown protocol-codegen mode: {other}");
            eprintln!("usage: protocol-codegen [--print|--write PATH|--check PATH]");
            std::process::exit(2);
        }
    }
}

fn required_path(value: Option<String>, mode: &str) -> PathBuf {
    value
        .map(PathBuf::from)
        .unwrap_or_else(|| panic!("{mode} requires a path"))
}
