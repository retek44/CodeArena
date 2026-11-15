import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        int n = Integer.parseInt(reader.readLine().trim());
        StringTokenizer prices = new StringTokenizer(reader.readLine());
        long low = Long.MAX_VALUE, best = 0;
        for (int i = 0; i < n; i++) {
            long p = Long.parseLong(prices.nextToken());
            if (p < low) low = p;
            else best = Math.max(best, p - low);
        }
        System.out.println(best);
    }
}
