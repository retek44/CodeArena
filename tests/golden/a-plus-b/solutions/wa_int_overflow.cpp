// Bug: int overflows for |a|, |b| up to 10^18.
#include <iostream>

int main() {
    long long a, b;
    std::cin >> a >> b;
    int sum = static_cast<int>(a + b);
    std::cout << sum << '\n';
}
