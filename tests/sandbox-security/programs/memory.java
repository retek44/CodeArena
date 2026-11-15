// Allocates far more than the heap limit and keeps it.
import java.util.*;

public class Main {
    public static void main(String[] args) {
        List<long[]> kept = new ArrayList<>();
        while (true) kept.add(new long[1 << 20]);
    }
}
