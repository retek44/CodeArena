#include <deque>
#include <iostream>
#include <string>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n, k;
    std::cin >> n >> k;
    std::vector<long long> values(n);
    for (auto& x : values) std::cin >> x;
    std::deque<int> window;
    std::string out;
    for (int i = 0; i < n; ++i) {
        while (!window.empty() && values[window.back()] <= values[i]) window.pop_back();
        window.push_back(i);
        if (window.front() <= i - k) window.pop_front();
        if (i >= k - 1) {
            if (i > k - 1) out += ' ';
            out += std::to_string(values[window.front()]);
        }
    }
    std::cout << out << '\n';
}
