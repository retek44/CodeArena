#include <cctype>
#include <iostream>
#include <string>

int main() {
    std::string line, kept;
    std::getline(std::cin, line);
    for (unsigned char ch : line)
        if (std::isalnum(ch)) kept += static_cast<char>(std::tolower(ch));
    std::string reversed(kept.rbegin(), kept.rend());
    std::cout << (kept == reversed ? "YES" : "NO") << '\n';
}
