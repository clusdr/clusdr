//! Membership snapshot, a worker lease, custom.hello, then the Watch bus.
//!
//! ```text
//! clusdr init && clusdr start --bootstrap
//! cargo run -p watch --manifest-path examples/Cargo.toml
//! cargo run -p watch --manifest-path examples/Cargo.toml -- --name edge-1
//! ```
//!
//! In another terminal: `clusdr publish ping '{"from":"cli"}'`

use clusdr::{local, Event, Member, Options, WatchFilter};
use std::env;
use std::process;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tokio_stream::StreamExt;

#[tokio::main]
async fn main() {
    let args = parse_args();
    let c = match local(Options::new()).await {
        Ok(c) => c,
        Err(e) => {
            eprintln!("connect: {e}");
            process::exit(1);
        }
    };

    if let Err(e) = snapshot(&c).await {
        eprintln!("snapshot: {e}");
        let _ = c.close().await;
        process::exit(1);
    }
    if args.once {
        let _ = c.close().await;
        return;
    }

    let lease_name = format!("worker.{}", args.name);
    match c.lease(&lease_name, Some(Duration::from_secs(15))).await {
        Ok(ls) => {
            println!(
                "lease {} owner={} token={} until {}",
                ls.name,
                ls.owner,
                ls.token,
                deadline_unix(ls.deadline())
            );
        }
        Err(e) => {
            eprintln!("lease: {e}");
            let _ = c.close().await;
            process::exit(1);
        }
    }

    let payload = format!(
        r#"{{"worker":"{}","pid":{},"lease":"{}"}}"#,
        args.name,
        process::id(),
        lease_name
    );
    if let Err(e) = c.publish("hello", payload.as_bytes()).await {
        eprintln!("publish: {e}");
        let _ = c.close().await;
        process::exit(1);
    }

    eprintln!("watching (Ctrl-C to stop); try: clusdr publish ping '{{\"from\":\"cli\"}}'");
    let mut events = match c.watch(WatchFilter::default()).await {
        Ok(s) => s,
        Err(e) => {
            eprintln!("watch: {e}");
            let _ = c.close().await;
            process::exit(1);
        }
    };
    loop {
        tokio::select! {
            _ = tokio::signal::ctrl_c() => break,
            next = events.next() => {
                match next {
                    Some(Ok(ev)) => print_event(&ev),
                    Some(Err(e)) => {
                        eprintln!("watch: {e}");
                        break;
                    }
                    None => break,
                }
            }
        }
    }
    let _ = c.close().await;
}

struct Args {
    name: String,
    once: bool,
}

fn parse_args() -> Args {
    let mut name = format!("pid-{}", process::id());
    let mut once = false;
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--name" | "-name" => name = need_value(&mut args, &arg),
            "-once" | "--once" => once = true,
            "-h" | "--help" => {
                eprintln!("usage: watch [--name NAME] [--once]");
                process::exit(0);
            }
            other => {
                eprintln!("unknown argument: {other}");
                process::exit(2);
            }
        }
    }
    Args { name, once }
}

fn need_value(args: &mut impl Iterator<Item = String>, flag: &str) -> String {
    args.next().unwrap_or_else(|| {
        eprintln!("missing value for {flag}");
        process::exit(2);
    })
}

async fn snapshot(c: &clusdr::Cluster) -> clusdr::Result<()> {
    let members = c.members().await?;
    println!(
        "{:<24} {:<22} {:<8} {:<10} LEADER",
        "ID", "ADDRESS", "STATUS", "ROLE"
    );
    for m in &members {
        println!(
            "{:<24} {:<22} {:<8} {:<10} {}",
            m.id,
            m.address,
            m.status,
            role(m),
            m.leader
        );
    }
    let leader = c.leader().await?;
    println!("leader {} at {}", leader.id, leader.address);
    Ok(())
}

fn role(m: &Member) -> &str {
    if m.role.is_empty() {
        "voter"
    } else {
        &m.role
    }
}

fn print_event(ev: &Event) {
    let kind = if matches!(ev.event_type.as_str(), "member.join" | "leader.changed") {
        "cluster"
    } else if ev.event_type == "member.dead" {
        "dead"
    } else if ev.event_type == "member.left" {
        "left"
    } else if ev.event_type.starts_with("custom.") {
        "gossip"
    } else if matches!(ev.event_type.as_str(), "watch.sync" | "watch.gap") {
        "watch"
    } else {
        "bus"
    };
    let ts = clock(ev.timestamp);
    let payload = if ev.payload.is_empty() {
        String::new()
    } else {
        format!(" {}", String::from_utf8_lossy(&ev.payload))
    };
    println!(
        "{kind} {ts} seq={} {} src={}{payload}",
        ev.seq, ev.event_type, ev.source
    );
}

fn clock(t: SystemTime) -> String {
    t.duration_since(UNIX_EPOCH)
        .map(|d| {
            let s = d.as_secs() % 86400;
            format!("{:02}:{:02}:{:02}", s / 3600, (s % 3600) / 60, s % 60)
        })
        .unwrap_or_default()
}

fn deadline_unix(t: Option<SystemTime>) -> String {
    t.and_then(|st| st.duration_since(UNIX_EPOCH).ok())
        .map(|d| d.as_secs().to_string())
        .unwrap_or_else(|| "?".into())
}
