import java.io.*;
import java.util.*;

// Dijkstra with 32-bit distances: five roads of 10^9 minutes overflow.
public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        StringTokenizer first = new StringTokenizer(reader.readLine());
        int n = Integer.parseInt(first.nextToken()), m = Integer.parseInt(first.nextToken());
        List<List<int[]>> roads = new ArrayList<>();
        for (int i = 0; i <= n; i++) roads.add(new ArrayList<>());
        for (int k = 0; k < m; k++) {
            StringTokenizer line = new StringTokenizer(reader.readLine());
            int a = Integer.parseInt(line.nextToken());
            int b = Integer.parseInt(line.nextToken());
            int w = Integer.parseInt(line.nextToken());
            roads.get(a).add(new int[] {b, w});
        }
        int[] dist = new int[n + 1];
        Arrays.fill(dist, Integer.MAX_VALUE);
        dist[1] = 0;
        PriorityQueue<int[]> queue = new PriorityQueue<>((x, y) -> Integer.compare(x[0], y[0]));
        queue.add(new int[] {0, 1});
        while (!queue.isEmpty()) {
            int[] cur = queue.poll();
            if (cur[0] > dist[cur[1]]) continue;
            for (int[] road : roads.get(cur[1])) {
                int d = cur[0] + road[1];
                if (d < dist[road[0]]) {
                    dist[road[0]] = d;
                    queue.add(new int[] {d, road[0]});
                }
            }
        }
        System.out.println(dist[n] == Integer.MAX_VALUE ? -1 : dist[n]);
    }
}
