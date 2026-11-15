#include <iostream>
#include <string>
#include <utility>
#include <vector>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    int r, c;
    std::cin >> r >> c;
    std::vector<std::string> grid(r);
    for (auto& line : grid) std::cin >> line;
    std::vector<char> seen(static_cast<std::size_t>(r) * c, 0);
    std::vector<std::pair<int, int>> stack;
    const int dy[] = {1, -1, 0, 0}, dx[] = {0, 0, 1, -1};
    int islands = 0;
    for (int i = 0; i < r; ++i)
        for (int j = 0; j < c; ++j) {
            if (grid[i][j] != '#' || seen[i * c + j]) continue;
            ++islands;
            seen[i * c + j] = 1;
            stack.push_back({i, j});
            while (!stack.empty()) {
                auto [y, x] = stack.back();
                stack.pop_back();
                for (int d = 0; d < 4; ++d) {
                    int ny = y + dy[d], nx = x + dx[d];
                    if (ny < 0 || ny >= r || nx < 0 || nx >= c) continue;
                    if (grid[ny][nx] != '#' || seen[ny * c + nx]) continue;
                    seen[ny * c + nx] = 1;
                    stack.push_back({ny, nx});
                }
            }
        }
    std::cout << islands << '\n';
}
