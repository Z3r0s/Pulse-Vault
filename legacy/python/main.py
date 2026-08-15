import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent / "src"))

from pulsevault.main import main

if __name__ == "__main__":
    main()
    # GUI polish for Ubuntu 26.04 Yaru applied in src/pulsevault/gui/* (Agent Beta)
