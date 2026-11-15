// The runner has no route out: a TCP connection must fail, and fail fast.
#include <arpa/inet.h>
#include <cstdio>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/time.h>
#include <unistd.h>

static const char* tcp(const char* ip, int port) {
    int fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) return "blocked";
    timeval timeout{2, 0};
    setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &timeout, sizeof timeout);
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    inet_pton(AF_INET, ip, &addr.sin_addr);
    int rc = connect(fd, reinterpret_cast<sockaddr*>(&addr), sizeof addr);
    close(fd);
    return rc == 0 ? "open" : "blocked";
}

int main() {
    std::printf("tcp: %s\n", tcp("1.1.1.1", 53));
    std::printf("tcp: %s\n", tcp("8.8.8.8", 443));
}
