#include <algorithm>
#include <cstdio>
#include <iostream>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n;
    std::cin >> n;
    std::vector<long long> values(n);
    long long sum = 0;
    for (auto& x : values) {
        std::cin >> x;
        sum += x;
    }
    std::sort(values.begin(), values.end());
    double median = n % 2 ? values[n / 2] : (values[n / 2 - 1] + values[n / 2]) / 2.0;
    std::printf("%.9f %.9f\n", static_cast<double>(sum) / n, median);
}
