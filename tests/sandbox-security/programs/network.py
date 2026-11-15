# The runner has no route out (isolated subnets on AWS, an internal Docker network in
# CI): connections and name lookups must fail, and fail fast.
import socket


def tcp(host: str, port: int) -> str:
    try:
        with socket.create_connection((host, port), timeout=2):
            return "open"
    except OSError:
        return "blocked"


def dns(name: str) -> str:
    try:
        socket.getaddrinfo(name, 443)
        return "resolved"
    except OSError:
        return "blocked"


print("tcp:", tcp("1.1.1.1", 53))
print("tcp:", tcp("8.8.8.8", 443))
print("dns:", dns("example.com"))
