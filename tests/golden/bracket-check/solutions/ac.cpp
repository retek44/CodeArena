#include <iostream>
#include <string>

int main() {
    std::string s;
    std::cin >> s;
    std::string stack;
    for (char ch : s) {
        if (ch == '(' || ch == '[' || ch == '{') {
            stack.push_back(ch);
            continue;
        }
        char open = ch == ')' ? '(' : ch == ']' ? '[' : '{';
        if (stack.empty() || stack.back() != open) {
            std::cout << "NO\n";
            return 0;
        }
        stack.pop_back();
    }
    std::cout << (stack.empty() ? "YES" : "NO") << '\n';
}
