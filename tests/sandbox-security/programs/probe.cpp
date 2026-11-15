// Runs after every C++ scenario: no leftover process of an earlier run may be alive,
// and the runner still judges a normal program (prints the sum of the input).
// The C++ runner serves only C++, so the probe counts every process other than itself
// whose command line is a program started from the scratch directory (./main).
#include <dirent.h>
#include <fstream>
#include <iostream>
#include <string>
#include <unistd.h>

int main() {
    int processes = 0;
    std::string self = std::to_string(getpid());
    if (DIR* proc = opendir("/proc")) {
        while (dirent* entry = readdir(proc)) {
            std::string pid = entry->d_name;
            if (pid.find_first_not_of("0123456789") != std::string::npos || pid == self) continue;
            std::ifstream cmdline("/proc/" + pid + "/cmdline");
            std::string argv0;
            std::getline(cmdline, argv0, '\0');
            if (argv0 == "./main") ++processes;
        }
        closedir(proc);
    }
    long long a, b;
    std::cin >> a >> b;
    std::cout << "processes: " << processes << "\n" << a + b << "\n";
}
