import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        int n = Integer.parseInt(reader.readLine().trim());
        StringTokenizer tokens = new StringTokenizer(reader.readLine());
        long[] values = new long[n];
        long sum = 0;
        for (int i = 0; i < n; i++) {
            values[i] = Long.parseLong(tokens.nextToken());
            sum += values[i];
        }
        Arrays.sort(values);
        double median = n % 2 == 1 ? values[n / 2] : (values[n / 2 - 1] + values[n / 2]) / 2.0;
        System.out.println(String.format(Locale.ROOT, "%.9f %.9f", (double) sum / n, median));
    }
}
