import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        StringTokenizer first = new StringTokenizer(reader.readLine());
        int n = Integer.parseInt(first.nextToken()), m = Integer.parseInt(first.nextToken());
        int[] head = new int[n + 1], next = new int[m], to = new int[m], waiting = new int[n + 1];
        Arrays.fill(head, -1);
        for (int k = 0; k < m; k++) {
            StringTokenizer line = new StringTokenizer(reader.readLine());
            int a = Integer.parseInt(line.nextToken());
            to[k] = Integer.parseInt(line.nextToken());
            waiting[to[k]]++;
            next[k] = head[a];
            head[a] = k;
        }
        int[] order = new int[n];
        int size = 0;
        for (int t = 1; t <= n; t++) if (waiting[t] == 0) order[size++] = t;
        for (int i = 0; i < size; i++)
            for (int k = head[order[i]]; k != -1; k = next[k])
                if (--waiting[to[k]] == 0) order[size++] = to[k];
        if (size != n) {
            System.out.println("IMPOSSIBLE");
            return;
        }
        StringBuilder out = new StringBuilder();
        for (int i = 0; i < n; i++) out.append(order[i]).append(i + 1 < n ? ' ' : '\n');
        System.out.print(out);
    }
}
