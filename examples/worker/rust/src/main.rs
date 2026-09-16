//! Holds a named lease the way a shard worker would.
//!
//! ```text
//! clusdr init && clusdr start --bootstrap
//! cargo run -p worker --manifest-path examples/Cargo.toml -- --name shard-7 --owner worker-a
//! cargo run -p worker --manifest-path examples/Cargo.toml -- --name shard-7 --owner worker-b
//! ```

use clusdr::{local, Options};
use std::env;
use std::process;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

#[tokio::main]
async fn main() {
    let args = parse_args();
    let opts = if args.owner.is_empty() {
        Options::new()
    } else {
        Options::new().holder(&args.owner)
    };
    let c = match local(opts).await {
        Ok(c) => c,
        Err(e) => {
            eprintln!("connect: {e}");
            process::exit(1);
        }
    };

    let ttl = if args.ttl.is_zero() {
        None
    } else {
        Some(args.ttl)
    };
    match c.lease(&args.name, ttl).await {
        Ok(ls) => {
            eprintln!(
                "granted name={} owner={} token={} until={}",
                ls.name,
                ls.owner,
                ls.token,
                deadline_unix(ls.deadline())
            );
            eprintln!("holding shard (Ctrl-C to stop; close revokes)");
        }
        Err(e) => {
            eprintln!("lease: {e}");
            let _ = c.close().await;
            process::exit(1);
        }
    }

    let _ = tokio::signal::ctrl_c().await;
    let _ = c.close().await;
}

struct Args {
    name: String,
    owner: String,
    ttl: Duration,
}

fn parse_args() -> Args {
    let mut name = "shard-7".to_string();
    let mut owner = String::new();
    let mut ttl = Duration::from_secs(15);
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--name" | "-name" => name = need_value(&mut args, &arg),
            "--owner" | "-owner" => owner = need_value(&mut args, &arg),
            "--ttl" | "-ttl" => {
                ttl = Duration::from_secs_f64(parse_f64(&need_value(&mut args, &arg), &arg))
            }
            "-h" | "--help" => {
                eprintln!("usage: worker [--name NAME] [--owner ID] [--ttl SECONDS]");
                process::exit(0);
            }
            other => {
                eprintln!("unknown argument: {other}");
                process::exit(2);
            }
        }
    }
    Args { name, owner, ttl }
}

fn need_value(args: &mut impl Iterator<Item = String>, flag: &str) -> String {
    args.next().unwrap_or_else(|| {
        eprintln!("missing value for {flag}");
        process::exit(2);
    })
}

fn parse_f64(raw: &str, flag: &str) -> f64 {
    raw.parse().unwrap_or_else(|_| {
        eprintln!("invalid number for {flag}: {raw}");
        process::exit(2);
    })
}

fn deadline_unix(t: Option<SystemTime>) -> String {
    t.and_then(|st| st.duration_since(UNIX_EPOCH).ok())
        .map(|d| d.as_secs().to_string())
        .unwrap_or_else(|| "?".into())
}
