"""Generate the icons from the master artwork in assets/.

Two things come out of it, both from the one master.

The Windows icon: assets/application-icon.png becomes a multi-size .ico beside
it. That single file is the whole identity. It is the icon on the agent and on
the setup program, which is in turn where the taskbar button, the notification
area icon and both shortcuts take theirs, because each of those reads the icon
out of the binary rather than carrying a copy of its own.

The setup window's header mark: the same master, trimmed and written small
enough for a page that has no bundler to load it as it finds it. The master is
over a megabyte, which is right for artwork and wrong for one badge.

Nothing here resamples the artwork upwards. Reframing is a crop or a pad; an
upscale invents detail the master does not have.

Run it when the master changes:

    python tools/genicons.py

It is not part of the build. The output is committed, so a clone needs neither
Python nor Pillow to build anything.
"""

from __future__ import annotations

import pathlib
import sys

try:
    from PIL import Image
except ImportError:  # pragma: no cover - a plain message beats a traceback
    sys.exit("Pillow is required: python -m pip install pillow")

# MASTER is the one piece of artwork everything else is derived from.
MASTER = "application-icon.png"

# ICO_SIZES are the sizes Windows chooses between: the notification area and
# menu sizes, the taskbar and shortcut sizes, then the large one Explorer uses
# in its biggest view. Leaving one out makes Windows scale a neighbour, which
# looks soft.
ICO_SIZES = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]

# HEADER_SIZE is the setup window's header mark. It is roughly twice the 126
# pixels it is drawn at, so it stays crisp on a high-density display without
# carrying detail nothing shows.
HEADER_SIZE = 256

REPO = pathlib.Path(__file__).resolve().parent.parent
MASTERS = REPO / "assets"
HEADER = REPO / "installer" / "frontend" / "dist" / "icon.png"


def trimmed(master: pathlib.Path) -> Image.Image:
    """Open the master and crop away the transparent margin around its artwork."""
    image = Image.open(master).convert("RGBA")
    box = image.getbbox()
    return image.crop(box) if box is not None else image


def write_ico(artwork: Image.Image, target: pathlib.Path) -> None:
    """Write the multi-size Windows icon."""
    artwork.save(target, format="ICO", sizes=ICO_SIZES)
    print(f"{target.relative_to(REPO)}  {target.stat().st_size:,} bytes")


def write_header(artwork: Image.Image, target: pathlib.Path) -> None:
    """Write the setup window's header mark."""
    target.parent.mkdir(parents=True, exist_ok=True)
    artwork.resize((HEADER_SIZE, HEADER_SIZE), Image.LANCZOS).save(target, optimize=True)
    print(f"{target.relative_to(REPO)}  {target.stat().st_size:,} bytes")


def main() -> int:
    """Generate every icon, reporting what was written."""
    master = MASTERS / MASTER
    if not master.exists():
        print(f"missing {master.relative_to(REPO)}", file=sys.stderr)
        return 1
    artwork = trimmed(master)
    write_ico(artwork, master.with_suffix(".ico"))
    write_header(artwork, HEADER)
    return 0


if __name__ == "__main__":
    sys.exit(main())
