import java.io.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        int q = Integer.parseInt(reader.readLine().trim());
        int[] queries = new int[q];
        int limit = 1;
        for (int i = 0; i < q; i++) {
            queries[i] = Integer.parseInt(reader.readLine().trim());
            limit = Math.max(limit, queries[i]);
        }
        boolean[] composite = new boolean[limit + 1];
        int[] count = new int[limit + 1];
        for (int i = 2; i <= limit; i++) {
            count[i] = count[i - 1];
            if (composite[i]) continue;
            count[i]++;
            for (long j = (long) i * i; j <= limit; j += i) composite[(int) j] = true;
        }
        StringBuilder out = new StringBuilder();
        for (int n : queries) out.append(count[n]).append('\n');
        System.out.print(out);
    }
}
