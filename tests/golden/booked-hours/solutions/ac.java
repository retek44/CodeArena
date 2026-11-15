import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        int n = Integer.parseInt(reader.readLine().trim());
        long[][] bookings = new long[n][2];
        for (int i = 0; i < n; i++) {
            StringTokenizer line = new StringTokenizer(reader.readLine());
            bookings[i][0] = Long.parseLong(line.nextToken());
            bookings[i][1] = Long.parseLong(line.nextToken());
        }
        Arrays.sort(bookings, (x, y) -> Long.compare(x[0], y[0]));
        StringBuilder out = new StringBuilder();
        long start = bookings[0][0], end = bookings[0][1];
        for (int i = 1; i < n; i++) {
            if (bookings[i][0] <= end) {
                end = Math.max(end, bookings[i][1]);
            } else {
                out.append(start).append(' ').append(end).append('\n');
                start = bookings[i][0];
                end = bookings[i][1];
            }
        }
        out.append(start).append(' ').append(end).append('\n');
        System.out.print(out);
    }
}
