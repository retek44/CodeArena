// Keeps a large table it never needs.
#include <iostream>
#include <vector>

int main() {
    std::vector<long long> table(40'000'000, 1);
    long long a, b;
    std::cin >> a >> b;
    std::cout << a + b + table[12345] - 1 << '\n';
}
