import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        int n = Integer.parseInt(reader.readLine().trim());
        StringTokenizer values = new StringTokenizer(reader.readLine());
        long best = Long.MIN_VALUE, current = 0;
        for (int i = 0; i < n; i++) {
            long x = Long.parseLong(values.nextToken());
            current = i == 0 ? x : Math.max(x, current + x);
            best = Math.max(best, current);
        }
        System.out.println(best);
    }
}
