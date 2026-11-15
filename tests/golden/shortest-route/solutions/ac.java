import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        DataInputStream in = new DataInputStream(new BufferedInputStream(System.in, 1 << 16));
        int n = (int) read(in), m = (int) read(in);
        int[] head = new int[n + 1], next = new int[m], to = new int[m];
        long[] weight = new long[m];
        Arrays.fill(head, -1);
        for (int k = 0; k < m; k++) {
            int a = (int) read(in);
            to[k] = (int) read(in);
            weight[k] = read(in);
            next[k] = head[a];
            head[a] = k;
        }
        long[] dist = new long[n + 1];
        Arrays.fill(dist, -1);
        dist[1] = 0;
        PriorityQueue<long[]> queue = new PriorityQueue<>((x, y) -> Long.compare(x[0], y[0]));
        queue.add(new long[] {0, 1});
        while (!queue.isEmpty()) {
            long[] cur = queue.poll();
            int town = (int) cur[1];
            if (cur[0] > dist[town]) continue;
            for (int k = head[town]; k != -1; k = next[k]) {
                long d = cur[0] + weight[k];
                if (dist[to[k]] == -1 || d < dist[to[k]]) {
                    dist[to[k]] = d;
                    queue.add(new long[] {d, to[k]});
                }
            }
        }
        System.out.println(dist[n]);
    }

    private static long read(DataInputStream in) throws IOException {
        int c = in.read();
        while (c == ' ' || c == '\n' || c == '\r') c = in.read();
        long v = 0;
        while (c >= '0' && c <= '9') {
            v = v * 10 + (c - '0');
            c = in.read();
        }
        return v;
    }
}
