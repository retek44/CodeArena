// Keeps allocating and touching memory: the memory limit must stop it (memory_limit).
#include <cstring>
#include <vector>

int main() {
    std::vector<char*> chunks;
    for (;;) {
        char* chunk = new char[8 * 1024 * 1024];
        std::memset(chunk, 1, 8 * 1024 * 1024);
        chunks.push_back(chunk);
    }
}
