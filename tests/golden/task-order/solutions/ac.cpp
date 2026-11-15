#include <iostream>
#include <string>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n, m;
    std::cin >> n >> m;
    std::vector<std::vector<int>> after(n + 1);
    std::vector<int> waiting(n + 1, 0);
    for (int k = 0; k < m; ++k) {
        int a, b;
        std::cin >> a >> b;
        after[a].push_back(b);
        ++waiting[b];
    }
    std::vector<int> order;
    order.reserve(n);
    for (int t = 1; t <= n; ++t)
        if (waiting[t] == 0) order.push_back(t);
    for (std::size_t i = 0; i < order.size(); ++i)
        for (int next : after[order[i]])
            if (--waiting[next] == 0) order.push_back(next);
    if (static_cast<int>(order.size()) != n) {
        std::cout << "IMPOSSIBLE\n";
        return 0;
    }
    std::string out;
    for (int i = 0; i < n; ++i) out += std::to_string(order[i]) + (i + 1 < n ? ' ' : '\n');
    std::cout << out;
}
