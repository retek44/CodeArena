// Starts many short-lived child processes: the process limit must refuse some of them.
// Each child only sleeps; the runner removes them after the run.
#include <cstdio>
#include <unistd.h>

int main() {
    int started = 0, refused = 0;
    for (int i = 0; i < 64; ++i) {
        pid_t pid = fork();
        if (pid < 0) {
            ++refused;
        } else if (pid == 0) {
            sleep(30);
            _exit(0);
        } else {
            ++started;
        }
    }
    if (refused > 0) {
        std::printf("limited\n");
    } else {
        std::printf("not limited: %d children started\n", started);
    }
}
