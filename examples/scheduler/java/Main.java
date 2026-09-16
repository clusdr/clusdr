/**
 * Hold one exclusive cluster lock and “dispatch” work.
 *
 * Run two copies with different --holder values. Only one replica holds
 * "scheduler" at a time. close() unlocks.
 *
 *     clusdr init && clusdr start --bootstrap
 *     mvn -q -f examples/scheduler/java/pom.xml exec:java -Dexec.args="--holder replica-a"
 *     mvn -q -f examples/scheduler/java/pom.xml exec:java -Dexec.args="--holder replica-b"
 *
 * tryLock returns empty when the name is taken (no current-holder object).
 */
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Lock;
import io.clusdr.Options;
import java.time.Duration;
import java.util.Optional;

public final class Main {
  public static void main(String[] args) {
    String name = arg(args, "--name", "scheduler");
    String holder = arg(args, "--holder", "");
    int work = num(args, "--work", 3);
    int wait = num(args, "--wait", 1);

    Cluster c;
    try {
      c = Clusdr.local(Options.defaults().holder(holder));
    } catch (ClusdrException e) {
      System.err.println("connect failed: " + e);
      System.exit(1);
      return;
    }

    System.err.println("scheduler replica lock=" + name + " work=" + work + "s");
    try {
      while (true) {
        shift(c, name, work, wait);
      }
    } catch (ClusdrException e) {
      System.err.println("shift failed: " + e.getMessage());
      System.exit(1);
    } finally {
      c.close();
    }
  }

  static void shift(Cluster c, String name, int work, int wait) {
    Optional<Lock> maybe = c.tryLock(name, Duration.ofSeconds(15), Duration.ofSeconds(5));
    if (maybe.isEmpty()) {
      System.err.println("waiting held_by=another replica");
      sleep(wait);
      return;
    }
    Lock lk = maybe.get();
    String until = lk.deadline() != null ? lk.deadline().toString() : "?";
    System.err.println(
        "held name=" + lk.name() + " holder=" + lk.holder() + " token=" + lk.token() + " until=" + until);
    System.out.println("dispatch job=rollout fence=" + lk.token() + " holder=" + lk.holder());
    sleep(work);
    c.unlock(name);
    System.err.println("released name=" + name);
    sleep(wait);
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

  static void sleep(int seconds) {
    try {
      Thread.sleep(seconds * 1000L);
    } catch (InterruptedException ie) {
      Thread.currentThread().interrupt();
      throw new ClusdrException("interrupted", ie);
    }
  }
}
