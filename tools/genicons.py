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

# HEADER_SIZE is the header mark. It is roughly twice the 126 pixels it is drawn
# at, so it stays crisp on a high-density display without carrying detail
# nothing shows.
HEADER_SIZE = 256

# BADGE_SIZE is the artwork inside a header button, drawn at 42 pixels, plus the
# profile picture on a row in the manager, drawn at 34. One size covers both at
# roughly three times the larger, which is what a high-density display wants.
BADGE_SIZE = 128

# DONATE_WIDTH is the donation artwork on its own panel, drawn at 220 pixels.
DONATE_WIDTH = 440

# PAGE_ICONS are the pictures a page loads as it finds them, since neither page
# has a bundler. Each is named by the file it comes from and the windows that
# want it: the marks and the theme artwork are shared, the profile picture and
# the donation artwork belong to the manager alone.
PAGE_ICONS = {
    "light-mode.png": ("manager", "setup"),
    "dark-mode.png": ("manager", "setup"),
    "help.png": ("manager",),
    "settings.png": ("manager",),
    "profile.png": ("manager",),
    "donate.png": ("manager",),
}

REPO = pathlib.Path(__file__).resolve().parent.parent
MASTERS = REPO / "assets"
PAGES = {
    "manager": REPO / "frontend" / "dist",
    "setup": REPO / "installer" / "frontend" / "dist",
}


def trimmed(master: pathlib.Path) -> Image.Image:
    """Open the master and crop away the transparent margin around its artwork."""
    image = Image.open(master).convert("RGBA")
    box = image.getbbox()
    return image.crop(box) if box is not None else image


def write_ico(artwork: Image.Image, target: pathlib.Path) -> None:
    """Write the multi-size Windows icon."""
    artwork.save(target, format="ICO", sizes=ICO_SIZES)
    print(f"{target.relative_to(REPO)}  {target.stat().st_size:,} bytes")


def write_page_image(artwork: Image.Image, target: pathlib.Path, size: int) -> None:
    """Write one square page image at the size the page draws it from."""
    target.parent.mkdir(parents=True, exist_ok=True)
    artwork.resize((size, size), Image.LANCZOS).save(target, optimize=True)
    print(f"{target.relative_to(REPO)}  {target.stat().st_size:,} bytes")


def write_wide_image(artwork: Image.Image, target: pathlib.Path, width: int) -> None:
    """Write one page image that is not square, keeping its proportions.

    The donation artwork is wider than it is tall. Squaring it would stretch a
    drawing somebody made, which is a worse answer than carrying two functions.
    """
    target.parent.mkdir(parents=True, exist_ok=True)
    height = max(1, round(artwork.height * width / artwork.width))
    artwork.resize((width, height), Image.LANCZOS).save(target, optimize=True)
    print(f"{target.relative_to(REPO)}  {target.stat().st_size:,} bytes")


def main() -> int:
    """Generate every icon, reporting what was written."""
    master = MASTERS / MASTER
    if not master.exists():
        print(f"missing {master.relative_to(REPO)}", file=sys.stderr)
        return 1
    artwork = trimmed(master)
    write_ico(artwork, master.with_suffix(".ico"))
    for page in PAGES.values():
        write_page_image(artwork, page / "icon.png", HEADER_SIZE)

    for name, windows in PAGE_ICONS.items():
        source = MASTERS / name
        if not source.exists():
            print(f"missing {source.relative_to(REPO)}", file=sys.stderr)
            return 1
        picture = trimmed(source)
        for window in windows:
            target = PAGES[window] / name
            if name == "donate.png":
                write_wide_image(picture, target, DONATE_WIDTH)
                continue
            write_page_image(picture, target, BADGE_SIZE)
    return 0


if __name__ == "__main__":
    sys.exit(main())
