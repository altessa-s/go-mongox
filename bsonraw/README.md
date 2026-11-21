# bsonraw

```go
import "github.com/altessa-s/go-mongox/bsonraw"
```

Walk reads a whole value, checks lengths and terminators, and visits nested elements without recursion. The visitor receives the depth, type and a key slice that aliases the input. Return an error to stop the walk.

## Usage

```go
err := bsonraw.Walk(bson.TypeEmbeddedDocument, raw, 1000, nil)
```

## Errors

ErrMalformed identifies invalid BSON structure. ErrTooDeep identifies the caller’s nesting limit. Visitor errors propagate unchanged. Use `errors.Is` for sentinel errors.
