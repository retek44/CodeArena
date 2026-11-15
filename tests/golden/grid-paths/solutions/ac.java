import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        final long mod = 1_000_000_007L;
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        StringTokenizer size = new StringTokenizer(reader.readLine());
        int r = Integer.parseInt(size.nextToken());
        int c = Integer.parseInt(size.nextToken());
        long[] row = new long[c];
        for (int i = 0; i < r; i++) {
            String line = reader.readLine().trim();
            long left = 0;
            for (int j = 0; j < c; j++) {
                if (line.charAt(j) == '#') left = 0;
                else if (i == 0 && j == 0) left = 1;
                else left = (left + row[j]) % mod;
                row[j] = left;
            }
        }
        System.out.println(row[c - 1]);
    }
}
