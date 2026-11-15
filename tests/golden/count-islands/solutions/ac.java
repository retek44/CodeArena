import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        StringTokenizer size = new StringTokenizer(reader.readLine());
        int r = Integer.parseInt(size.nextToken()), c = Integer.parseInt(size.nextToken());
        char[][] grid = new char[r][];
        for (int i = 0; i < r; i++) grid[i] = reader.readLine().trim().toCharArray();
        boolean[] seen = new boolean[r * c];
        int[] stack = new int[r * c];
        int[] dy = {1, -1, 0, 0}, dx = {0, 0, 1, -1};
        int islands = 0;
        for (int i = 0; i < r; i++)
            for (int j = 0; j < c; j++) {
                if (grid[i][j] != '#' || seen[i * c + j]) continue;
                islands++;
                seen[i * c + j] = true;
                int top = 0;
                stack[top++] = i * c + j;
                while (top > 0) {
                    int cell = stack[--top], y = cell / c, x = cell % c;
                    for (int d = 0; d < 4; d++) {
                        int ny = y + dy[d], nx = x + dx[d];
                        if (ny < 0 || ny >= r || nx < 0 || nx >= c) continue;
                        if (grid[ny][nx] != '#' || seen[ny * c + nx]) continue;
                        seen[ny * c + nx] = true;
                        stack[top++] = ny * c + nx;
                    }
                }
            }
        System.out.println(islands);
    }
}
