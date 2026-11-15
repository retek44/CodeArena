#include <iostream>
#include <string>
#include <vector>

// Forgets the modulus: the count wraps around 2^64 on large grids.
int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int r, c;
    std::cin >> r >> c;
    std::vector<unsigned long long> row(c, 0);
    std::string line;
    for (int i = 0; i < r; ++i) {
        std::cin >> line;
        unsigned long long left = 0;
        for (int j = 0; j < c; ++j) {
            if (line[j] == '#') left = 0;
            else if (i == 0 && j == 0) left = 1;
            else left = left + row[j];
            row[j] = left;
        }
    }
    std::cout << row[c - 1] << '\n';
}
