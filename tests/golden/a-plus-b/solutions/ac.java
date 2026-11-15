import java.io.*;
import java.util.*;

public class Main {
    public static void main(String[] args) throws IOException {
        StringTokenizer in = new StringTokenizer(new BufferedReader(new InputStreamReader(System.in)).readLine());
        long a = Long.parseLong(in.nextToken());
        long b = Long.parseLong(in.nextToken());
        System.out.println(a + b);
    }
}
