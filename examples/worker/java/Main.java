/**
 * Hold a named lease the way a shard worker would.
 *
 * A lease does not wait. If the name is taken, grant fails immediately
 * (unlike scheduler, which retries tryLock). close() revokes.
 *
 *     clusdr init && clusdr start --bootstrap
 *     mvn -q -f examples/worker/java/pom.xml exec:java -Dexec.args="--name shard-7 --owner worker-a"
 *     mvn -q -f examples/worker/java/pom.xml exec:java -Dexec.args="--name shard-7 --owner worker-b"
 */
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Lease;
import io.clusdr.Options;
import java.time.Duration;

public final class Main {
  public static void main(String[] args) {
    String name = arg(args, "--name", "shard-7");
    String owner = arg(args, "--owner", "");
    int ttl = num(args, "--ttl", 15);

    Cluster c;
    try {
      c = Clusdr.local(Options.defaults().holder(owner));
    } catch (ClusdrException e) {
      System.err.println("connect failed: " + e);
      System.exit(1);
      return;
    }

    try {
      Lease ls = c.lease(name, Duration.ofSeconds(ttl));
      String until = ls.deadline() != null ? ls.deadline().toString() : "?";
      System.err.println(
          "granted name=" + ls.name() + " owner=" + ls.owner() + " token=" + ls.token() + " until=" + until);
      System.err.println("holding shard (Ctrl-C to stop; close revokes)");
      while (true) {
        Thread.sleep(3600_000L);
      }
    } catch (InterruptedException ie) {
      Thread.currentThread().interrupt();
      System.err.println("stopping");
    } catch (ClusdrException e) {
      System.err.println("lease failed: " + e.getMessage());
      System.exit(1);
    } finally {
      c.close();
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
