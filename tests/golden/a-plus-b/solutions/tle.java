// Counts up one by one: correct, but far too slow for big numbers.
import java.util.*;

public class Main {
    static volatile long total;

    public static void main(String[] args) {
        Scanner in = new Scanner(System.in);
        long a = in.nextLong();
        long b = in.nextLong();
        total = a;
        long step = b >= 0 ? 1 : -1;
        for (long i = 0; i < Math.abs(b); i++) total += step;
        System.out.println(total);
    }
}
