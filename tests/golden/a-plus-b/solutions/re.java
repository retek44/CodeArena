// Reads past the end of an empty array.
import java.util.*;

public class Main {
    public static void main(String[] args) {
        long[] values = new long[0];
        Scanner in = new Scanner(System.in);
        long a = in.nextLong();
        long b = in.nextLong();
        System.out.println(values[5] + a + b);
    }
}
