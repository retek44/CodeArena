#include <algorithm>
#include <iostream>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n;
    std::cin >> n;
    long long best = 0, current = 0;
    for (int i = 0; i < n; ++i) {
        long long x;
        std::cin >> x;
        current = i == 0 ? x : std::max(x, current + x);
        best = i == 0 ? current : std::max(best, current);
    }
    std::cout << best << '\n';
}
