//! Prints cluster membership and then follows Watch.
//!
//! ```text
//! clusdr init && clusdr start --bootstrap
//! cargo run -p who --manifest-path examples/Cargo.toml
//! cargo run -p who --manifest-path examples/Cargo.toml -- --once
//! ```

use clusdr::{local, Event, Member, Options, WatchFilter};
use std::env;
use std::process;
use tokio_stream::StreamExt;

#[tokio::main]
async fn main() {
    let once = parse_once();
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
    if once {
        let _ = c.close().await;
        return;
    }

    eprintln!("watching cluster events (Ctrl-C to stop)");
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

fn parse_once() -> bool {
    let mut once = false;
    for arg in env::args().skip(1) {
        match arg.as_str() {
            "-once" | "--once" => once = true,
            "-h" | "--help" => {
                eprintln!("usage: who [--once]");
                process::exit(0);
            }
            other => {
                eprintln!("unknown argument: {other}");
                process::exit(2);
            }
        }
    }
    once
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
    let kind = if matches!(
        ev.event_type.as_str(),
        "member.join" | "member.left" | "leader.changed"
    ) {
        "cluster"
    } else if ev.event_type.starts_with("custom.") {
        "gossip"
    } else if matches!(ev.event_type.as_str(), "watch.sync" | "watch.gap") {
        "watch"
    } else {
        "bus"
    };
    let payload = if ev.payload.is_empty() {
        String::new()
    } else {
        format!(" {}", String::from_utf8_lossy(&ev.payload))
    };
    println!(
        "{kind} seq={} {} src={}{payload}",
        ev.seq, ev.event_type, ev.source
    );
}
