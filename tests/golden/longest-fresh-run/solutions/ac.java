import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        String s = new BufferedReader(new InputStreamReader(System.in)).readLine().trim();
        int[] last = new int[128];
        Arrays.fill(last, -1);
        int left = 0, best = 0;
        for (int i = 0; i < s.length(); i++) {
            char ch = s.charAt(i);
            if (last[ch] >= left) left = last[ch] + 1;
            last[ch] = i;
            best = Math.max(best, i - left + 1);
        }
        System.out.println(best);
    }
}
