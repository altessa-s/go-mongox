# go-mongox

| Package | Purpose |
| --- | --- |
| [bsoncodec](./bsoncodec) | Encodes optional values, exact decimals and calendar dates |
| [bsonfilter](./bsonfilter) | Validates filter values and prepares executable bson |
| [keyset](./keyset) | Reads signed keyset pages from mongodb collections |
| [indexes](./indexes) | Resolves index names and creates indexes |
| [bsonbound](./bsonbound) | Limits the output of bson encoders |
| [bsonraw](./bsonraw) | Walks the structure of a bson value |

## Library rules

- Use published go-aip root, go-atlas-core, mongo-driver v2 and shopspring/decimal. Keep the module independent of both consumer adapters.
- Validate the Go input before canonicalization and bounded encoding. Bind and execute the checked bytes.
- Preserve BSON types, ordered document keys and historical signer bindings. Keep filter values and tokens out of error messages.
- Index creation is non-destructive. Consumers own index reconciliation and collection lifecycle.
- Add doc.go, README.md and black-box tests per package. Benchmark hot paths, use t.Context() and b.Loop(), and fuzz byte parsers.
- Generate DateOption using pinned optgen and make generate. Keep generated output reproducible.
- Gate live server tests with MONGO_URI. Create isolated test collections and clean up only data created by the test.
- Run make lint, make vet, make test-all and make fuzz. Follow CONTRIBUTING.md for formatting, scopes and commits.
