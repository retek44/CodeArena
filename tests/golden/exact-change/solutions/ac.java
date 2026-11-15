import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        StringTokenizer first = new StringTokenizer(reader.readLine());
        int k = Integer.parseInt(first.nextToken()), a = Integer.parseInt(first.nextToken());
        StringTokenizer values = new StringTokenizer(reader.readLine());
        int[] coins = new int[k];
        for (int i = 0; i < k; i++) coins[i] = Integer.parseInt(values.nextToken());
        int[] best = new int[a + 1];
        for (int v = 1; v <= a; v++) {
            best[v] = a + 1;
            for (int coin : coins)
                if (coin <= v && best[v - coin] + 1 < best[v]) best[v] = best[v - coin] + 1;
        }
        System.out.println(best[a] > a ? -1 : best[a]);
    }
}
