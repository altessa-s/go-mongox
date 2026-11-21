# Working on go-mongox

Read [AGENTS.md](./AGENTS.md) for the package map and library rules and [CONTRIBUTING.md](./CONTRIBUTING.md) for the project checks.

Keep changes within the requested module. Use US English for code and documentation. Run one shell command at a time. Preserve user changes and create commits only when requested.

| Package | Purpose |
| --- | --- |
| [bsoncodec](./bsoncodec) | Encodes optional values, exact decimals and calendar dates |
| [bsonfilter](./bsonfilter) | Validates filter values and prepares executable bson |
| [keyset](./keyset) | Reads signed keyset pages from mongodb collections |
| [indexes](./indexes) | Resolves index names and creates indexes |
| [bsonbound](./bsonbound) | Limits the output of bson encoders |
| [bsonraw](./bsonraw) | Walks the structure of a bson value |
