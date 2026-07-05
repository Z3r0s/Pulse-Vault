#!/usr/bin/env python3
"""Simple changelog section extractor for Keep a Changelog format.
Usage: python tmp_changelog_extract.py [version]  # version like 0.3.0 or v0.3.0 or 'Unreleased'
Outputs the body for that section.
"""
import re
import sys
from pathlib import Path

def extract_section(changelog_path: str, version: str) -> str:
    content = Path(changelog_path).read_text(encoding="utf-8")
    ver = version.lstrip('vV')
    if ver.lower() == 'unreleased':
        pattern = r'## \[Unreleased\](.*?)(?=\n## \[|\Z)'
    else:
        # Match [0.3.0] or [v0.3.0] or 0.3.0 etc, with optional date
        pattern = rf'## \[(?:v)?{re.escape(ver)}\].*?(.*?)(?=\n## \[|\Z)'
    match = re.search(pattern, content, re.DOTALL | re.IGNORECASE)
    if match:
        body = match.group(1).strip()
        # Clean leading newlines/headers
        body = re.sub(r'^\s*\n+', '', body)
        return body
    # Fallback: first section after title
    match = re.search(r'## \[([^\]]+)\].*?(.*?)(?=\n## \[|\Z)', content, re.DOTALL)
    if match:
        return match.group(2).strip()
    return "See CHANGELOG.md for details."

if __name__ == "__main__":
    ver = sys.argv[1] if len(sys.argv) > 1 else "Unreleased"
    print(extract_section("CHANGELOG.md", ver))
