import java.io.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        String a = reader.readLine().trim(), b = reader.readLine().trim();
        int[] prev = new int[b.length() + 1], cur = new int[b.length() + 1];
        for (int j = 0; j <= b.length(); j++) prev[j] = j;
        for (int i = 1; i <= a.length(); i++) {
            cur[0] = i;
            for (int j = 1; j <= b.length(); j++)
                cur[j] = a.charAt(i - 1) == b.charAt(j - 1)
                        ? prev[j - 1]
                        : 1 + Math.min(prev[j - 1], Math.min(prev[j], cur[j - 1]));
            int[] swap = prev;
            prev = cur;
            cur = swap;
        }
        System.out.println(prev[b.length()]);
    }
}
