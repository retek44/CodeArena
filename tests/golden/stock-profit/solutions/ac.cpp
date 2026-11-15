#include <iostream>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n;
    std::cin >> n;
    long long low = 0, best = 0;
    for (int i = 0; i < n; ++i) {
        long long p;
        std::cin >> p;
        if (i == 0 || p < low) low = p;
        else if (p - low > best) best = p - low;
    }
    std::cout << best << '\n';
}
