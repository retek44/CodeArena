# Keeps allocating and touching memory: the memory limit must stop it (memory_limit).
chunks = []
while True:
    chunks.append(bytearray(8 * 1024 * 1024))
