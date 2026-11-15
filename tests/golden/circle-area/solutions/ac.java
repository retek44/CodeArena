import java.io.*;

public class Main {
    public static void main(String[] args) throws IOException {
        double r = Double.parseDouble(new BufferedReader(new InputStreamReader(System.in)).readLine().trim());
        System.out.println(String.format("%.10f", Math.PI * r * r));
    }
}
