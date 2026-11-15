// The runner has no route out: a TCP connection must fail, and fail fast.
import java.net.*;

public class Main {
    static String tcp(String host, int port) {
        try (Socket socket = new Socket()) {
            socket.connect(new InetSocketAddress(InetAddress.getByAddress(parse(host)), port), 2000);
            return "open";
        } catch (Exception e) {
            return "blocked";
        }
    }

    static byte[] parse(String ip) {
        String[] parts = ip.split("\\.");
        byte[] bytes = new byte[4];
        for (int i = 0; i < 4; i++) bytes[i] = (byte) Integer.parseInt(parts[i]);
        return bytes;
    }

    public static void main(String[] args) {
        System.out.println("tcp: " + tcp("1.1.1.1", 53));
        System.out.println("tcp: " + tcp("8.8.8.8", 443));
    }
}
