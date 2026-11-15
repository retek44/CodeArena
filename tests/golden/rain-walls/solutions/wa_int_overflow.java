import java.io.*;
import java.util.*;

// Adds the water up in 32 bits: three gaps of 10^9 overflow.
public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        int n = Integer.parseInt(reader.readLine().trim());
        StringTokenizer tokens = new StringTokenizer(reader.readLine());
        int[] h = new int[n];
        for (int i = 0; i < n; i++) h[i] = Integer.parseInt(tokens.nextToken());
        int left = 0, right = n - 1, leftMax = 0, rightMax = 0, water = 0;
        while (left < right) {
            if (h[left] < h[right]) {
                leftMax = Math.max(leftMax, h[left]);
                water += leftMax - h[left++];
            } else {
                rightMax = Math.max(rightMax, h[right]);
                water += rightMax - h[right--];
            }
        }
        System.out.println(water);
    }
}
