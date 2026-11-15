# The program gets a fixed, minimal environment: nothing of the runner's (AWS
# credentials, Lambda variables) is passed on.
import os

print(" ".join(sorted(os.environ)))
