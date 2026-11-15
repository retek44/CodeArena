import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        StringTokenizer first = new StringTokenizer(reader.readLine());
        int n = Integer.parseInt(first.nextToken()), k = Integer.parseInt(first.nextToken());
        StringTokenizer tokens = new StringTokenizer(reader.readLine());
        long[] values = new long[n];
        for (int i = 0; i < n; i++) values[i] = Long.parseLong(tokens.nextToken());
        int[] window = new int[n];
        int head = 0, tail = 0;
        StringBuilder out = new StringBuilder();
        for (int i = 0; i < n; i++) {
            while (tail > head && values[window[tail - 1]] <= values[i]) tail--;
            window[tail++] = i;
            if (window[head] <= i - k) head++;
            if (i >= k - 1) {
                if (i > k - 1) out.append(' ');
                out.append(values[window[head]]);
            }
        }
        System.out.println(out);
    }
}
