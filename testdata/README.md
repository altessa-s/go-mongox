# MongoDB migration fixtures

The files were generated with the reference Atlas codecs and keyset binding and position functions at commit d59f630a, using driver v2.10.1 and pagetoken v0.1.8.

- `codecs.bson` contains an ordered document. Both cloud.google.com/go/civil.Date and atlas-core civil.Date hold 2024-02-29 and their respective zero values. Optional and pointer fields use decimal 12.50. Int32 and Int64 both hold 42. Neutral tests explicitly register a cloud-shaped date and compare the entire document to the real cloud-codec output.
- `tokens-sorted.json` and `tokens-reversed.json` represent the two historical encodings of the same nested map. Hex fields contain the complete filter and position payloads. Tokens use the signed pagetoken codec, a 32-byte test key filled with 0x42, clock 2026-01-02T03:04:05Z, a 24h TTL, subject fixture-reader and 1ms precision. The sort is rank descending followed by identity.seq ascending. The position is Int32(7), Int64(42).
- An ordered filter reproduces either token unchanged. For a map, canonical preparation matches the sorted token and the reversed token fails with ErrFilterChanged. A coefficient larger than 4096 bits can be representable after decimal normalization in legacy Atlas. The neutral codec rejects it before normalization.

`raw-filters.json` lists complete raw filters as hex with expected bsonfilter error names. An empty error name means the exact input bytes must be returned.
