"""Regenerate the embedded English frequency table.

Requires: pip install wordfreq
Run: python3 gen.py  (from this directory; writes ../data/en_top50k.tsv.gz)

Takes wordfreq's top 50k English words with Zipf values. Deterministic
output: sorted by Zipf desc, then word asc; Zipf rounded to 2 decimals.
"""
import gzip
import sys

from wordfreq import top_n_list, zipf_frequency

N = 50000

def main() -> None:
    words = top_n_list("en", N)
    rows = [(w, round(zipf_frequency(w, "en"), 2)) for w in words]
    rows.sort(key=lambda r: (-r[1], r[0]))
    with gzip.open("../data/en_top50k.tsv.gz", "wt", encoding="utf-8") as f:
        for w, z in rows:
            f.write(f"{w}\t{z:.2f}\n")
    print(f"wrote {len(rows)} rows, zipf {rows[0][1]:.2f}..{rows[-1][1]:.2f}")

if __name__ == "__main__":
    sys.exit(main())
