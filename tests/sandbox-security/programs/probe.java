// Runs after every Java scenario: no leftover JVM of an earlier run may be alive, and
// the runner still judges a normal program (prints the sum of the input).
import java.util.*;

public class Main {
    public static void main(String[] args) {
        long self = ProcessHandle.current().pid();
        long processes = ProcessHandle.allProcesses()
            .filter(p -> p.pid() != self)
            .filter(p -> p.info().command().map(c -> c.endsWith("/java")).orElse(false))
            .count();
        Scanner in = new Scanner(System.in);
        long a = in.nextLong();
        long b = in.nextLong();
        System.out.println("processes: " + processes);
        System.out.println(a + b);
    }
}
