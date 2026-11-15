#include <iostream>
#include <string>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    const long long mod = 1000000007;
    int r, c;
    std::cin >> r >> c;
    std::vector<long long> row(c, 0);
    std::string line;
    for (int i = 0; i < r; ++i) {
        std::cin >> line;
        long long left = 0;
        for (int j = 0; j < c; ++j) {
            if (line[j] == '#') left = 0;
            else if (i == 0 && j == 0) left = 1;
            else left = (left + row[j]) % mod;
            row[j] = left;
        }
    }
    std::cout << row[c - 1] << '\n';
}
