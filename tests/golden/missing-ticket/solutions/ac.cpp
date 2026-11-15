#include <iostream>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    long long n;
    std::cin >> n;
    long long missing = n * (n + 1) / 2;
    for (long long i = 1; i < n; ++i) {
        long long x;
        std::cin >> x;
        missing -= x;
    }
    std::cout << missing << '\n';
}
