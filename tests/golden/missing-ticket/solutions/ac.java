import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        int n = Integer.parseInt(reader.readLine().trim());
        long missing = (long) n * (n + 1) / 2;
        String line = reader.readLine();
        if (line != null) {
            StringTokenizer numbers = new StringTokenizer(line);
            while (numbers.hasMoreTokens()) missing -= Long.parseLong(numbers.nextToken());
        }
        System.out.println(missing);
    }
}
