"""Check every relative link and heading anchor in the repo's markdown.

The repo is 27% markdown and its documents cite each other by path and by anchor —
docs/TECH.md's sections above all. A moved file or a renamed heading breaks those silently,
and the reader who finds it is the one who needed the number.

Deliberately dependency-free and offline. External URLs are not fetched: a gate that needs
the network is a gate that fails on the machine this project exists to work offline on, and
a link rotting on someone else's server is not this repo drifting.
"""

import pathlib
import re
import subprocess
import sys

LINK = re.compile(r"\[[^\]]*\]\(([^)\s]+)\)")
HEADING = re.compile(r"^#{1,6}\s+(.*?)\s*$")


def anchor(heading):
    """GitHub's slug: lowercase, punctuation dropped, spaces to hyphens."""
    return re.sub(r"[^a-z0-9 -]", "", heading.lower()).replace(" ", "-")


def main():
    root = pathlib.Path(
        subprocess.run(["git", "rev-parse", "--show-toplevel"],
                       capture_output=True, text=True, check=True).stdout.strip())
    tracked = subprocess.run(["git", "ls-files", "*.md"],
                             capture_output=True, text=True, check=True).stdout.split()
    docs = [root / f for f in tracked]
    anchors = {
        d: {anchor(m.group(1)) for m in map(HEADING.match, d.read_text().splitlines()) if m}
        for d in docs
    }

    broken = []
    for doc in docs:
        for n, line in enumerate(doc.read_text().splitlines(), 1):
            for m in LINK.finditer(line):
                target = m.group(1)
                if target.startswith(("http://", "https://", "mailto:")):
                    continue
                path, _, frag = target.partition("#")
                dest = doc if not path else (doc.parent / path).resolve()
                where = f"{doc.relative_to(root)}:{n}"
                if path and not dest.exists():
                    broken.append(f"{where}: {target} — no such file")
                elif frag and frag not in anchors.get(dest, set()):
                    # A link into a non-markdown file cannot carry an anchor either.
                    broken.append(f"{where}: {target} — no such heading")

    for b in broken:
        print(b, file=sys.stderr)
    print(f"doclinks: {len(docs)} files checked, {len(broken)} broken")
    return 1 if broken else 0


if __name__ == "__main__":
    sys.exit(main())
