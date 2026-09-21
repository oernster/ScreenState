"""Stamp the version from VERSION into the ScreenState site.

A browser rendering the site cannot read VERSION, so every place the site shows a
version carries a delimited token: <!--VERSION-->x.y.z<!--/VERSION-->. This script
rewrites whatever sits between the delimiters. VERSION stays the one place a real
version string is written by hand.

The site only, deliberately: docs/**/*.html and docs/**/*.md. Markdown at the
repository root holds no version by rule, so it is never a target.

Idempotent. A file already carrying the current version is left alone rather than
rewritten, so a second run changes nothing and says so. Files are read and written as
bytes, so their line endings survive a stamp on any platform.

A missing or empty VERSION is a failure rather than a sentinel: the build calls this
script and has to stop instead of publishing a number nobody chose.
"""

from __future__ import annotations

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent
VERSION_FILE = ROOT / "VERSION"
SITE_DIR = ROOT / "docs"
SITE_PATTERNS = ("**/*.html", "**/*.md")
OPEN_TOKEN = "<!--VERSION-->"
CLOSE_TOKEN = "<!--/VERSION-->"
TOKEN = re.compile(re.escape(OPEN_TOKEN) + r".*?" + re.escape(CLOSE_TOKEN), re.DOTALL)
ENCODING = "utf-8"
SUCCESS = 0
FAILURE = 1


def read_version() -> str | None:
    """The version VERSION holds; None when the file is missing or empty."""
    try:
        text = VERSION_FILE.read_text(encoding=ENCODING).strip()
    except OSError:
        return None
    return text or None


def site_files() -> list[pathlib.Path]:
    """Every site page that could carry a token, in a stable order."""
    found: set[pathlib.Path] = set()
    for pattern in SITE_PATTERNS:
        found.update(path for path in SITE_DIR.glob(pattern) if path.is_file())
    return sorted(found)


def stamp(path: pathlib.Path, version: str) -> bool:
    """Put this version in every token in one file; True when the file changed."""
    original = path.read_bytes().decode(ENCODING)
    stamped = TOKEN.sub(lambda _match: f"{OPEN_TOKEN}{version}{CLOSE_TOKEN}", original)
    if stamped == original:
        return False
    path.write_bytes(stamped.encode(ENCODING))
    return True


def main() -> int:
    """Stamp the whole site, naming each file actually touched."""
    version = read_version()
    if version is None:
        print(f"no version in {VERSION_FILE}; refusing to stamp", file=sys.stderr)
        return FAILURE
    if not SITE_DIR.is_dir():
        print(f"no site at {SITE_DIR}; nothing to stamp")
        return SUCCESS
    touched = [path for path in site_files() if stamp(path, version)]
    if not touched:
        print(f"nothing to stamp: the site is already at {version}")
        return SUCCESS
    print(f"stamped {version} into:")
    for path in touched:
        print(f"  {path.relative_to(ROOT).as_posix()}")
    return SUCCESS


if __name__ == "__main__":
    sys.exit(main())
