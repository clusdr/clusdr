/**
 * Watch the local daemon: membership snapshot, a worker lease, then the bus.
 *
 *     clusdr init && clusdr start --bootstrap
 *     mvn -q -f examples/watch/java/pom.xml exec:java -Dexec.args="--name edge-1"
 *
 * In another terminal:
 *
 *     clusdr publish ping '{"from":"cli"}'
 *
 * custom.* is gossip (not Raft, not replayed). member.dead is crash (still listed).
 * member.left is clusdr leave (gone).
 * close() revokes the worker lease.
 */
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Event;
import io.clusdr.Lease;
import io.clusdr.Member;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

public final class Main {
  public static void main(String[] args) {
    String name = arg(args, "--name", "pid-" + ProcessHandle.current().pid());
    boolean once = Arrays.asList(args).contains("--once");

    Cluster c;
    try {
      c = Clusdr.local();
    } catch (ClusdrException e) {
      System.err.println("connect failed: " + e);
      System.err.println(
          "need a running daemon and TLS certs in ~/.clusdr (or CLUSDR_TLS=disabled on daemon and client)");
      System.exit(1);
      return;
    }

    String leaseName = "worker." + name;
    try {
      printMembers(c);
      if (once) {
        return;
      }
      Lease ls = c.lease(leaseName, Duration.ofSeconds(15));
      String until = ls.deadline() != null ? ls.deadline().toString() : "?";
      System.out.println(
          "lease " + ls.name() + " owner=" + ls.owner() + " token=" + ls.token() + " until " + until);
      c.publish("hello", Map.of("worker", name, "pid", ProcessHandle.current().pid(), "lease", leaseName));
      System.out.println("watching (Ctrl-C to stop); try: clusdr publish ping '{\"from\":\"cli\"}'");
      for (Event ev : c.watch()) {
        System.out.println(formatEvent(ev));
      }
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    } finally {
      c.close();
    }
  }

  static void printMembers(Cluster c) {
    List<Member> members = c.members();
    System.out.printf("%-24s %-22s %-8s %-10s %s%n", "ID", "ADDRESS", "STATUS", "ROLE", "LEADER");
    for (Member m : members) {
      String role = m.role().isEmpty() ? "voter" : m.role();
      System.out.printf("%-24s %-22s %-8s %-10s %s%n", m.id(), m.address(), m.status(), role, m.leader());
    }
    Member leader = c.leader();
    System.out.println("leader " + leader.id() + " at " + leader.address());
  }

  static String formatEvent(Event ev) {
    String kind = "bus";
    if (ev.type().equals("member.join") || ev.type().equals("leader.changed")) {
      kind = "cluster";
    } else if (ev.type().equals("member.dead")) {
      kind = "dead";
    } else if (ev.type().equals("member.left")) {
      kind = "left";
    } else if (ev.type().startsWith("custom.")) {
      kind = "gossip";
    } else if (ev.type().equals("watch.sync") || ev.type().equals("watch.gap")) {
      kind = "watch";
    }
    String ts = DateTimeFormatter.ofPattern("HH:mm:ss").withZone(ZoneOffset.UTC).format(ev.timestamp());
    String payload = "";
    if (ev.payload().length > 0) {
      payload = " " + new String(ev.payload(), StandardCharsets.UTF_8);
    }
    return kind + " " + ts + " seq=" + ev.seq() + " " + ev.type() + " src=" + ev.source() + payload;
  }

  static String arg(String[] args, String name, String fallback) {
    for (int i = 0; i < args.length - 1; i++) {
      if (args[i].equals(name)) {
        return args[i + 1];
      }
    }
    return fallback;
  }
}
