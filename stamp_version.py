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

Every page's local stylesheet and script links are versioned by content too: each
carries ?v=<hash> of the file it names, so a deploy that changes the file changes its
address and no browser pairs a new page with a cached old stylesheet. A link to a file
that does not exist is a failure that names the path.
"""

from __future__ import annotations

import hashlib
import pathlib
import re
import sys
from urllib.parse import unquote

ROOT = pathlib.Path(__file__).resolve().parent
VERSION_FILE = ROOT / "VERSION"
SITE_DIR = ROOT / "docs"
SITE_PATTERNS = ("**/*.html", "**/*.md")
OPEN_TOKEN = "<!--VERSION-->"
CLOSE_TOKEN = "<!--/VERSION-->"
TOKEN = re.compile(re.escape(OPEN_TOKEN) + r".*?" + re.escape(CLOSE_TOKEN), re.DOTALL)
ENCODING = "utf-8"
# A local stylesheet or script link plus any query it already carries. A colon in the
# path means a scheme, so absolute URLs never match; root-absolute and
# protocol-relative paths are left alone by version_assets.
PAGE_PATTERN = "**/*.html"
ASSET_HASH_LENGTH = 10
ASSET_LINK = re.compile(
    r'\b(?P<attribute>href|src)="(?P<path>[^"?#:]+\.(?:css|js))(?:\?[^"#]*)?"'
)
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


def asset_hash(path: pathlib.Path) -> str:
    """The short content hash of one asset, reading CRLF as LF.

    A Windows checkout and the LF blob GitHub serves then agree, so a run on another
    machine does not rewrite every page.
    """
    content = path.read_bytes().replace(b"\r\n", b"\n")
    return hashlib.sha256(content).hexdigest()[:ASSET_HASH_LENGTH]


def version_assets(page: pathlib.Path) -> bool:
    """Put ?v=<hash> on every local asset link in one page; True when it changed."""
    original = page.read_bytes().decode(ENCODING)

    def versioned(match: re.Match[str]) -> str:
        link = match.group("path")
        if link.startswith("/"):
            return match.group(0)
        asset = page.parent / unquote(link)
        if not asset.is_file():
            raise FileNotFoundError(f"{page} links {link}; {asset} does not exist")
        return f'{match.group("attribute")}="{link}?v={asset_hash(asset)}"'

    stamped = ASSET_LINK.sub(versioned, original)
    if stamped == original:
        return False
    page.write_bytes(stamped.encode(ENCODING))
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
    else:
        print(f"stamped {version} into:")
        for path in touched:
            print(f"  {path.relative_to(ROOT).as_posix()}")
    try:
        pages = sorted(SITE_DIR.glob(PAGE_PATTERN))
        versioned = [page for page in pages if version_assets(page)]
    except FileNotFoundError as error:
        print(f"{error}; refusing to stamp", file=sys.stderr)
        return FAILURE
    if not versioned:
        print("nothing to version: every asset link already carries its hash")
        return SUCCESS
    print("versioned asset links in:")
    for page in versioned:
        print(f"  {page.relative_to(ROOT).as_posix()}")
    return SUCCESS


if __name__ == "__main__":
    sys.exit(main())
