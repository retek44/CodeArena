import java.io.*;

public class Main {
    public static void main(String[] args) throws IOException {
        String s = new BufferedReader(new InputStreamReader(System.in)).readLine().trim();
        char[] stack = new char[s.length()];
        int top = 0;
        for (char ch : s.toCharArray()) {
            if (ch == '(' || ch == '[' || ch == '{') {
                stack[top++] = ch;
                continue;
            }
            char open = ch == ')' ? '(' : ch == ']' ? '[' : '{';
            if (top == 0 || stack[top - 1] != open) {
                System.out.println("NO");
                return;
            }
            top--;
        }
        System.out.println(top == 0 ? "YES" : "NO");
    }
}
