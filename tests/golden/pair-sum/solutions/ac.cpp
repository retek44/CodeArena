#include <iostream>
#include <unordered_map>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n;
    long long t;
    std::cin >> n >> t;
    std::vector<long long> a(n);
    for (auto& x : a) std::cin >> x;
    std::unordered_map<long long, int> first;
    first.reserve(n * 2);
    for (int j = 0; j < n; ++j) {
        auto it = first.find(t - a[j]);
        if (it != first.end()) {
            std::cout << it->second + 1 << ' ' << j + 1 << '\n';
            return 0;
        }
        first.emplace(a[j], j);
    }
}
