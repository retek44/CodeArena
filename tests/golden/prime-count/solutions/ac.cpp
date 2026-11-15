#include <algorithm>
#include <iostream>
#include <string>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int q;
    std::cin >> q;
    std::vector<int> queries(q);
    for (auto& n : queries) std::cin >> n;
    int limit = *std::max_element(queries.begin(), queries.end());
    std::vector<char> composite(limit + 1, 0);
    std::vector<int> count(limit + 1, 0);
    for (long long i = 2; i <= limit; ++i) {
        count[i] = count[i - 1] + !composite[i];
        if (!composite[i])
            for (long long j = i * i; j <= limit; j += i) composite[j] = 1;
    }
    std::string out;
    for (int n : queries) out += std::to_string(count[n]) + '\n';
    std::cout << out;
}
