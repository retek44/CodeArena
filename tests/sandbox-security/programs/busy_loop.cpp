// Burns CPU forever: the CPU time limit must stop it (time_limit).
int main() {
    volatile unsigned long long n = 0;
    for (;;) {
        n = n + 1;
    }
}
