#include <algorithm>
#include <iostream>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int n;
    std::cin >> n;
    std::vector<long long> h(n);
    for (auto& x : h) std::cin >> x;
    int left = 0, right = n - 1;
    long long left_max = 0, right_max = 0, water = 0;
    while (left < right) {
        if (h[left] < h[right]) {
            left_max = std::max(left_max, h[left]);
            water += left_max - h[left++];
        } else {
            right_max = std::max(right_max, h[right]);
            water += right_max - h[right--];
        }
    }
    std::cout << water << '\n';
}
