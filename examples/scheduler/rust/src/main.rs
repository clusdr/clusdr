//! Holds one exclusive cluster lock and “dispatches” work.
//!
//! ```text
//! clusdr init && clusdr start --bootstrap
//! cargo run -p scheduler --manifest-path examples/Cargo.toml -- --holder replica-a
//! cargo run -p scheduler --manifest-path examples/Cargo.toml -- --holder replica-b
//! ```

use clusdr::{local, Options};
use std::env;
use std::process;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

#[tokio::main]
async fn main() {
    let args = parse_args();
    let opts = if args.holder.is_empty() {
        Options::new()
    } else {
        Options::new().holder(&args.holder)
    };
    let c = match local(opts).await {
        Ok(c) => c,
        Err(e) => {
            eprintln!("connect: {e}");
            process::exit(1);
        }
    };

    eprintln!(
        "scheduler replica lock={} work={}s",
        args.name,
        args.work.as_secs_f64()
    );
    loop {
        tokio::select! {
            _ = tokio::signal::ctrl_c() => break,
            result = shift(&c, &args) => {
                if let Err(e) = result {
                    eprintln!("shift: {e}");
                    let _ = c.close().await;
                    process::exit(1);
                }
            }
        }
    }
    let _ = c.close().await;
}

struct Args {
    name: String,
    holder: String,
    work: Duration,
    wait: Duration,
}

fn parse_args() -> Args {
    let mut name = "scheduler".to_string();
    let mut holder = String::new();
    let mut work = Duration::from_secs(3);
    let mut wait = Duration::from_secs(1);
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--name" | "-name" => name = need_value(&mut args, &arg),
            "--holder" | "-holder" => holder = need_value(&mut args, &arg),
            "--work" | "-work" => {
                work = Duration::from_secs_f64(parse_f64(&need_value(&mut args, &arg), &arg))
            }
            "--wait" | "-wait" => {
                wait = Duration::from_secs_f64(parse_f64(&need_value(&mut args, &arg), &arg))
            }
            "-h" | "--help" => {
                eprintln!("usage: scheduler [--name NAME] [--holder ID] [--work SECONDS] [--wait SECONDS]");
                process::exit(0);
            }
            other => {
                eprintln!("unknown argument: {other}");
                process::exit(2);
            }
        }
    }
    Args {
        name,
        holder,
        work,
        wait,
    }
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

async fn shift(c: &clusdr::Cluster, args: &Args) -> clusdr::Result<()> {
    let lk = c
        .try_lock(&args.name, Some(Duration::from_secs(15)))
        .await?;
    let Some(lk) = lk else {
        eprintln!("waiting held_by=another replica");
        tokio::time::sleep(args.wait).await;
        return Ok(());
    };

    eprintln!(
        "held name={} holder={} token={} until={}",
        lk.name,
        lk.holder,
        lk.token,
        deadline_unix(lk.deadline())
    );
    println!(
        "dispatch job=rollout fence={} holder={}",
        lk.token, lk.holder
    );
    tokio::time::sleep(args.work).await;
    c.unlock(&args.name).await?;
    eprintln!("released name={}", args.name);
    tokio::time::sleep(args.wait).await;
    Ok(())
}

fn deadline_unix(t: Option<SystemTime>) -> String {
    t.and_then(|st| st.duration_since(UNIX_EPOCH).ok())
        .map(|d| d.as_secs().to_string())
        .unwrap_or_else(|| "?".into())
}
