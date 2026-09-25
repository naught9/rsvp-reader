# Frequency data attribution

`en_top50k.tsv.gz` is derived from the [wordfreq](https://github.com/rspeer/wordfreq)
English word list (top 50,000 words with Zipf frequencies), by Robyn Speer.
wordfreq is available under the Apache License 2.0; its data carries the
project's attribution requirement, satisfied by this file: the word
frequencies combine multiple public sources (books, web, subtitles, news,
and more — see wordfreq's documentation for the full source list).

Regenerate with (requires `pip install wordfreq`):

```bash
cd internal/vocab/gen && python3 gen.py
```

The derived table (word + Zipf value, sorted by frequency) is a factual
extraction used only to pace rare words; application code stays under this
repository's MIT license.
