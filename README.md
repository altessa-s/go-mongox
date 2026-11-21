# go-mongox

MongoDB primitives for Go 1.27 and mongo-driver v2, shared by go-aip and go-atlas.

| Package | Purpose |
| --- | --- |
| [bsoncodec](./bsoncodec) | Encodes optional values, exact decimals and calendar dates |
| [bsonfilter](./bsonfilter) | Validates filter values and prepares executable bson |
| [keyset](./keyset) | Reads signed keyset pages from mongodb collections |
| [indexes](./indexes) | Resolves index names and creates indexes |
| [bsonbound](./bsonbound) | Limits the output of bson encoders |
| [bsonraw](./bsonraw) | Walks the structure of a bson value |

## Filter execution

```go
raw, err := bsonfilter.Prepare(ctx, doc, registry, bsonOptions)
if err != nil {
    return err
}
// Use the prepared document for the read.
result := collection.FindOne(ctx, raw)
```

## Tests

Run `go test -race ./...`. Set `MONGO_URI` to run server tests in isolated test collections. Fixed BSON and signed tokens for consumer migration tests live in [testdata](./testdata).
