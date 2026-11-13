#!/usr/bin/env python3
"""Tabulate TLC results written by run_tlc.sh.

  summarize.py <out-dir> [<out-dir> ...]

Each out-dir holds tlc_<cfg>.out files and the copied <cfg>.cfg; one Markdown
table row is printed per configuration (replicas / Byzantine / bounds, result,
distinct states, depth, time).
"""
import glob
import os
import re
import sys


def row(out_file):
    name = os.path.basename(out_file)[4:-4]
    text = open(out_file, errors="replace").read()
    cfg = {}
    cfg_file = os.path.join(os.path.dirname(out_file), name + ".cfg")
    if os.path.exists(cfg_file):
        for line in open(cfg_file):
            m = re.match(r"\s+(\w+) = (.*)", line)
            if m:
                cfg[m.group(1)] = m.group(2).strip()
    n = len(re.findall(r"r\d+", cfg.get("Replica", "")))
    byz = len(re.findall(r"r\d+", cfg.get("Byz", "")))
    err = re.search(r"Error: Invariant (\w+) is violated", text)
    if err:
        result = "violation: %s" % err.group(1)
    elif "No error has been found" in text:
        result = "no violation"
    elif "Finished in" in text:
        result = "error / stopped"
    else:
        result = "running"
    m = re.search(r"(\d+) states generated, (\d+) distinct states found", text)
    distinct = m.group(2) if m else ""
    if not m:
        p = re.findall(r"Progress\((\d+)\).*?([\d,]+) states generated.*?([\d,]+) distinct states", text)
        if p:
            distinct = p[-1][2] + " (in progress)"
    d = re.search(r"depth of the complete state graph search is (\d+)", text)
    t = re.search(r"Finished in (.+?) at", text)
    bounds = "%d / %d / F=%s" % (n, byz, cfg.get("F", "?"))
    if "FB" in cfg:
        bounds += " / FB=" + cfg["FB"]
    mode = []
    if cfg.get("LeaderOnly") == "TRUE":
        mode.append("leader-only")
    if cfg.get("ByzSign") == "FALSE":
        mode.append("Byzantine withhold")
    if cfg.get("CheckQuorum") == "FALSE":
        mode.append("no quorum check")
    if cfg.get("UseSeal") == "FALSE":
        mode.append("no seal")
    return (name, bounds, cfg.get("FirstFamily", "?").strip('"'),
            "%s / %s" % (cfg.get("MaxSeg", "?"), cfg.get("MaxPos", "?")),
            ", ".join(mode) or "-", result, distinct,
            d.group(1) if d else "", t.group(1) if t else "")


def main():
    rows = []
    for out_dir in sys.argv[1:]:
        for f in sorted(glob.glob(os.path.join(out_dir, "tlc_*.out"))):
            rows.append(row(f))
    print("| configuration | N / Byz / bounds | first family | segments / positions | mode | result | distinct states | depth | time |")
    print("|---|---|---|---|---|---|---|---|---|")
    for r in rows:
        print("| `%s` | %s | %s | %s | %s | %s | %s | %s | %s |" % r)


if __name__ == "__main__":
    main()
