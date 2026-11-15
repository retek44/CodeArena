#include <algorithm>
#include <iostream>
#include <string>
#include <utility>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n;
    std::cin >> n;
    std::vector<std::pair<long long, long long>> bookings(n);
    for (auto& [s, e] : bookings) std::cin >> s >> e;
    std::sort(bookings.begin(), bookings.end());
    std::vector<std::pair<long long, long long>> merged;
    for (auto [s, e] : bookings) {
        if (!merged.empty() && s <= merged.back().second)
            merged.back().second = std::max(merged.back().second, e);
        else
            merged.push_back({s, e});
    }
    std::string out;
    for (auto [s, e] : merged) out += std::to_string(s) + ' ' + std::to_string(e) + '\n';
    std::cout << out;
}
