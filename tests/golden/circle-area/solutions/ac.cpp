#include <cmath>
#include <cstdio>

int main() {
    double r;
    if (std::scanf("%lf", &r) != 1) return 1;
    std::printf("%.9f\n", std::acos(-1.0) * r * r);
}
