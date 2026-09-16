//! Agents signalling each other over the local daemon — no broker.
//!
//! ```text
//! clusdr init && clusdr start --bootstrap
//! cargo run -p agent --manifest-path examples/Cargo.toml -- --mode listen
//! cargo run -p agent --manifest-path examples/Cargo.toml -- --mode emit --from mapper
//! ```

use clusdr::{local, Options, WatchFilter};
use std::env;
use std::process;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tokio_stream::StreamExt;

const TOPIC: &str = "agent.task";

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

    if args.mode == "emit" || args.mode == "both" {
        let emitter = c.clone();
        let source = args.source.clone();
        let every = args.every;
        tokio::spawn(async move {
            emit_loop(emitter, source, every).await;
        });
        println!(
            "emitting custom.{TOPIC} every {}s from={}",
            args.every.as_secs_f64(),
            args.source
        );
    }

    if args.mode == "listen" || args.mode == "both" {
        println!("listening for custom.{TOPIC} (Ctrl-C to stop)");
        let mut events = match c.watch(WatchFilter::new().topics([TOPIC])).await {
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
                        Some(Ok(ev)) => {
                            let body = String::from_utf8_lossy(&ev.payload);
                            let ts = clock(ev.timestamp);
                            println!(
                                "{ts} seq={} {} src={} {body}",
                                ev.seq, ev.event_type, ev.source
                            );
                        }
                        Some(Err(e)) => {
                            eprintln!("watch: {e}");
                            break;
                        }
                        None => break,
                    }
                }
            }
        }
    } else {
        eprintln!("emitting (Ctrl-C to stop)");
        let _ = tokio::signal::ctrl_c().await;
    }
    let _ = c.close().await;
}

async fn emit_loop(c: clusdr::Cluster, source: String, every: Duration) {
    let mut n = 0u64;
    loop {
        tokio::time::sleep(every).await;
        n += 1;
        let payload = format!(r#"{{"from":"{source}","n":{n},"pid":{}}}"#, process::id());
        if let Err(e) = c.publish(TOPIC, payload.as_bytes()).await {
            eprintln!("publish failed: {e}");
            return;
        }
        println!("sent n={n} from={source}");
    }
}

struct Args {
    mode: String,
    source: String,
    every: Duration,
}

fn parse_args() -> Args {
    let mut mode = "both".to_string();
    let mut source = format!("pid-{}", process::id());
    let mut every = Duration::from_secs(2);
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--mode" | "-mode" => mode = need_value(&mut args, &arg),
            "--from" | "-from" => source = need_value(&mut args, &arg),
            "--every" | "-every" => {
                every = Duration::from_secs_f64(parse_f64(&need_value(&mut args, &arg), &arg))
            }
            "-h" | "--help" => {
                eprintln!("usage: agent [--mode listen|emit|both] [--from NAME] [--every SECONDS]");
                process::exit(0);
            }
            other => {
                eprintln!("unknown argument: {other}");
                process::exit(2);
            }
        }
    }
    if !matches!(mode.as_str(), "listen" | "emit" | "both") {
        eprintln!("mode must be listen, emit, or both");
        process::exit(2);
    }
    Args {
        mode,
        source,
        every,
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

fn clock(t: SystemTime) -> String {
    t.duration_since(UNIX_EPOCH)
        .map(|d| {
            let s = d.as_secs() % 86400;
            format!("{:02}:{:02}:{:02}", s / 3600, (s % 3600) / 60, s % 60)
        })
        .unwrap_or_default()
}
