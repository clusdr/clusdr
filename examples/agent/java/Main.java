/**
 * Agents signalling each other over the local daemon — no broker.
 *
 * Publish is 1-hop gossip, not Raft. It is not a queue. A subscriber
 * that connects later will not see old custom events.
 *
 *     clusdr init && clusdr start --bootstrap
 *     mvn -q -f examples/agent/java/pom.xml exec:java -Dexec.args="--mode listen"
 *     mvn -q -f examples/agent/java/pom.xml exec:java -Dexec.args="--mode emit --from mapper"
 */
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Event;
import io.clusdr.WatchFilter;
import java.nio.charset.StandardCharsets;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;

public final class Main {
  static final String TOPIC = "agent.task";

  public static void main(String[] args) {
    String mode = arg(args, "--mode", "both");
    String source = arg(args, "--from", "pid-" + ProcessHandle.current().pid());
    int every = num(args, "--every", 2);

    Cluster c;
    try {
      c = Clusdr.local();
    } catch (ClusdrException e) {
      System.err.println("connect failed: " + e);
      System.exit(1);
      return;
    }

    AtomicBoolean stop = new AtomicBoolean(false);
    try {
      if (mode.equals("emit") || mode.equals("both")) {
        Thread emitter = new Thread(() -> emitLoop(c, source, every, stop), "agent-emit");
        emitter.setDaemon(true);
        emitter.start();
        System.out.println("emitting custom." + TOPIC + " every " + every + "s from=" + source);
      }
      if (mode.equals("listen") || mode.equals("both")) {
        System.out.println("listening for custom." + TOPIC + " (Ctrl-C to stop)");
        for (Event ev : c.watch(WatchFilter.all().topics(TOPIC))) {
          String body = ev.payload().length > 0 ? new String(ev.payload(), StandardCharsets.UTF_8) : "";
          String ts = DateTimeFormatter.ofPattern("HH:mm:ss").withZone(ZoneOffset.UTC).format(ev.timestamp());
          System.out.println(ts + " seq=" + ev.seq() + " " + ev.type() + " src=" + ev.source() + " " + body);
        }
      } else {
        System.out.println("emitting (Ctrl-C to stop)");
        while (!stop.get()) {
          Thread.sleep(250);
        }
      }
    } catch (InterruptedException ie) {
      Thread.currentThread().interrupt();
      System.err.println("stopping");
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    } finally {
      stop.set(true);
      c.close();
    }
  }

  static void emitLoop(Cluster c, String source, int every, AtomicBoolean stop) {
    int n = 0;
    while (!stop.get()) {
      try {
        Thread.sleep(every * 1000L);
      } catch (InterruptedException ie) {
        Thread.currentThread().interrupt();
        return;
      }
      if (stop.get()) {
        return;
      }
      n += 1;
      try {
        c.publish(TOPIC, Map.of("from", source, "n", n, "pid", ProcessHandle.current().pid()));
      } catch (ClusdrException e) {
        System.err.println("publish failed: " + e.getMessage());
        return;
      }
      System.out.println("sent n=" + n + " from=" + source);
    }
  }

  static String arg(String[] args, String name, String fallback) {
    for (int i = 0; i < args.length - 1; i++) {
      if (args[i].equals(name)) {
        return args[i + 1];
      }
    }
    return fallback;
  }

  static int num(String[] args, String name, int fallback) {
    try {
      return Integer.parseInt(arg(args, name, String.valueOf(fallback)));
    } catch (NumberFormatException e) {
      return fallback;
    }
  }
}
