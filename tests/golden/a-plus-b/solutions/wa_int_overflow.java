// Bug: int overflows for |a|, |b| up to 10^18.
import java.util.*;

public class Main {
    public static void main(String[] args) {
        Scanner in = new Scanner(System.in);
        long a = in.nextLong();
        long b = in.nextLong();
        int sum = (int) (a + b);
        System.out.println(sum);
    }
}
