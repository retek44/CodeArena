import java.io.*;
import java.util.*;

// Sums in 32 bits: six days of 10^9 overflow.
public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        int n = Integer.parseInt(reader.readLine().trim());
        StringTokenizer values = new StringTokenizer(reader.readLine());
        int best = Integer.MIN_VALUE, current = 0;
        for (int i = 0; i < n; i++) {
            int x = Integer.parseInt(values.nextToken());
            current = i == 0 ? x : Math.max(x, current + x);
            best = Math.max(best, current);
        }
        System.out.println(best);
    }
}
