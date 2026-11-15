#include <algorithm>
#include <iostream>
#include <string>
#include <vector>

int main() {
    std::string a, b;
    std::cin >> a >> b;
    std::vector<int> prev(b.size() + 1), cur(b.size() + 1);
    for (std::size_t j = 0; j <= b.size(); ++j) prev[j] = static_cast<int>(j);
    for (std::size_t i = 1; i <= a.size(); ++i) {
        cur[0] = static_cast<int>(i);
        for (std::size_t j = 1; j <= b.size(); ++j)
            cur[j] = a[i - 1] == b[j - 1] ? prev[j - 1] : 1 + std::min({prev[j - 1], prev[j], cur[j - 1]});
        std::swap(prev, cur);
    }
    std::cout << prev[b.size()] << '\n';
}
