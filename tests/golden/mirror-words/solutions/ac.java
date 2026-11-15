import java.io.*;

public class Main {
    public static void main(String[] args) throws IOException {
        String line = new BufferedReader(new InputStreamReader(System.in)).readLine();
        StringBuilder kept = new StringBuilder();
        for (char ch : line.toCharArray())
            if (Character.isLetterOrDigit(ch)) kept.append(Character.toLowerCase(ch));
        String forwards = kept.toString();
        System.out.println(forwards.equals(kept.reverse().toString()) ? "YES" : "NO");
    }
}
