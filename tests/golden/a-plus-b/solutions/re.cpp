// Reads past the end of an empty vector.
#include <iostream>
#include <vector>

int main() {
    std::vector<long long> v;
    long long a, b;
    std::cin >> a >> b;
    std::cout << v.at(5) + a + b << '\n';
}
