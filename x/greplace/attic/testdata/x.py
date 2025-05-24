import sys
k = 65536 - 5
if len(sys.argv) > 1:
    k = int(sys.argv[1])
print(" " * k, end="")
print("computer")
