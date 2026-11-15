import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        StringTokenizer first = new StringTokenizer(reader.readLine());
        int n = Integer.parseInt(first.nextToken());
        long t = Long.parseLong(first.nextToken());
        StringTokenizer values = new StringTokenizer(reader.readLine());
        Map<Long, Integer> seen = new HashMap<>();
        for (int j = 0; j < n; j++) {
            long x = Long.parseLong(values.nextToken());
            Integer i = seen.get(t - x);
            if (i != null) {
                System.out.println((i + 1) + " " + (j + 1));
                return;
            }
            seen.putIfAbsent(x, j);
        }
    }
}
