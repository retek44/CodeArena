#include <functional>
#include <iostream>
#include <queue>
#include <utility>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n, m;
    std::cin >> n >> m;
    std::vector<std::vector<std::pair<int, long long>>> roads(n + 1);
    for (int k = 0; k < m; ++k) {
        int a, b;
        long long w;
        std::cin >> a >> b >> w;
        roads[a].push_back({b, w});
    }
    const long long inf = -1;
    std::vector<long long> dist(n + 1, inf);
    using Item = std::pair<long long, int>;
    std::priority_queue<Item, std::vector<Item>, std::greater<Item>> heap;
    dist[1] = 0;
    heap.push({0, 1});
    while (!heap.empty()) {
        auto [d, town] = heap.top();
        heap.pop();
        if (d > dist[town]) continue;
        for (auto [next, w] : roads[town]) {
            if (dist[next] == inf || d + w < dist[next]) {
                dist[next] = d + w;
                heap.push({d + w, next});
            }
        }
    }
    std::cout << dist[n] << '\n';
}
