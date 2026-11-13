// Counts up one by one: correct, but far too slow for big numbers.
#include <cstdlib>
#include <iostream>

int main() {
    long long a, b;
    std::cin >> a >> b;
    volatile long long total = a;
    long long step = b >= 0 ? 1 : -1;
    for (long long i = 0; i < std::llabs(b); ++i) total = total + step;
    std::cout << total << '\n';
}
