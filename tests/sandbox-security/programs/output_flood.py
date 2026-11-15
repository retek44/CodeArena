# Writes without end: the output limit must stop it (output_limit).
import sys

line = "x" * 4095 + "\n"
while True:
    sys.stdout.write(line)
