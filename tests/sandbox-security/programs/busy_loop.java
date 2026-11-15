// Burns CPU forever: the CPU time limit must stop it.
public class Main {
    static volatile long counter;

    public static void main(String[] args) {
        while (true) counter++;
    }
}
