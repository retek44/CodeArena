// Keeps a large table it never needs.
import java.util.*;

public class Main {
    public static void main(String[] args) {
        long[] table = new long[40_000_000];
        Arrays.fill(table, 1);
        Scanner in = new Scanner(System.in);
        long a = in.nextLong();
        long b = in.nextLong();
        System.out.println(a + b + table[12345] - 1);
    }
}
