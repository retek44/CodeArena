#include <iostream>
#include <vector>

int main() {
    int k, a;
    std::cin >> k >> a;
    std::vector<int> coins(k);
    for (auto& coin : coins) std::cin >> coin;
    const int inf = a + 1;
    std::vector<int> best(a + 1, inf);
    best[0] = 0;
    for (int v = 1; v <= a; ++v)
        for (int coin : coins)
            if (coin <= v && best[v - coin] + 1 < best[v]) best[v] = best[v - coin] + 1;
    std::cout << (best[a] > a ? -1 : best[a]) << '\n';
}
