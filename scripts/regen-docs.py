#!/usr/bin/env python3
"""Regenerate every captured `$ browserctrl list|find ...` block in the docs.

Why it exists: terminal blocks in the docs must be real output, and a change
to the table layout touches a dozen of them across three files. Recapturing
by hand missed four blocks once, so the README advertised output the binary
no longer produced. This rewrites each block from the doc fixture in place,
and is idempotent: a second run changes nothing.

Only plain ``` fences whose first line is `$ browserctrl ...` are touched,
and within them only `list` / `find` commands; ```sh / ```json blocks and
skills/--version captures are copied through. Run via `task docs:regen`,
which builds both binaries first.
"""
FIXTURE_BIN = "./bin/docfixture"
import re, shlex, subprocess, sys
FILES = ["README.md", "docs/commands.md", "docs/recipes.md"]
def capture(args):
    r = subprocess.run([FIXTURE_BIN, ".docs-fixture", "./bin/browserctrl"] + args,
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    return r.stdout.rstrip("\n")
total = 0
for path in FILES:
    lines = open(path).read().split("\n")
    out, i, changed = [], 0, 0
    while i < len(lines):
        if lines[i].startswith("```"):              # any fence opens a block
            opener = lines[i]
            j = i + 1
            while j < len(lines) and lines[j] != "```":
                j += 1
            block = lines[i + 1:j]
            if opener != "```":                     # ```sh / ```json: copy through
                out += [opener] + block + ["```"]
                i = j + 1
                continue
            if block and block[0].startswith("$ browserctrl "):
                segs, cur = [], None                # split on "$ " lines
                for ln in block:
                    if ln.startswith("$ "):
                        cur = [ln]; segs.append(cur)
                    else:
                        cur.append(ln)
                new_block = []
                for k, seg in enumerate(segs):
                    cmd = seg[0][len("$ browserctrl "):]
                    args = shlex.split(cmd)
                    if args and args[0] in ("list", "find"):
                        body = capture(args).split("\n")
                        new_seg = [seg[0]] + (body if body != [""] else [])
                        if k < len(segs) - 1:
                            new_seg.append("")      # blank line between commands
                    else:
                        new_seg = seg
                    if new_seg != seg:
                        changed += 1
                    new_block += new_seg
                block = new_block
            out += ["```"] + block + ["```"]
            i = j + 1
        else:
            out.append(lines[i]); i += 1
    open(path, "w").write("\n".join(out))
    print(f"{path}: {changed} block segment(s) regenerated")
    total += changed
print("total", total)
