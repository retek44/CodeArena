#include <algorithm>
#include <iostream>
#include <string>
#include <vector>

int main() {
    std::string s;
    std::cin >> s;
    std::vector<int> last(128, -1);
    int left = 0, best = 0;
    for (int i = 0; i < static_cast<int>(s.size()); ++i) {
        unsigned char ch = s[i];
        if (last[ch] >= left) left = last[ch] + 1;
        last[ch] = i;
        best = std::max(best, i - left + 1);
    }
    std::cout << best << '\n';
}
