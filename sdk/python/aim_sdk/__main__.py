"""Allow `python -m aim_sdk <command>` when the console script is not on PATH."""

import sys

from .cli import main

if __name__ == "__main__":
    sys.exit(main())
