/**
 * Print cluster membership, then follow Watch until Ctrl-C.
 *
 * The process is an application. It does not vote. It talks only to the
 * daemon on this host (CLUSDR_GRPC_ADDR or 127.0.0.1:7947).
 *
 *     clusdr init && clusdr start --bootstrap
 *     mvn -q -f examples/who/java/pom.xml exec:java
 *     mvn -q -f examples/who/java/pom.xml exec:java -Dexec.args=--once
 */
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Event;
import io.clusdr.Member;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.List;

public final class Main {
  public static void main(String[] args) {
    boolean once = Arrays.asList(args).contains("--once");
    Cluster c;
    try {
      c = Clusdr.local();
    } catch (ClusdrException e) {
      System.err.println("connect failed: " + e);
      System.exit(1);
      return;
    }
    try {
      printMembers(c);
      if (once) {
        return;
      }
      System.err.println("watching cluster events (Ctrl-C to stop)");
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
    String payload = "";
    if (ev.payload().length > 0) {
      payload = " " + new String(ev.payload(), StandardCharsets.UTF_8);
    }
    return kind + " seq=" + ev.seq() + " " + ev.type() + " src=" + ev.source() + payload;
  }
}
