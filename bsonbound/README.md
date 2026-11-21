# bsonbound

```go
import "github.com/altessa-s/go-mongox/bsonbound"
```

Marshal bounds the bytes and the number of open documents while encoding. NewValueWriter accepts an optional evaluator budget and returns a Bound whose Size and Depth track progress. Configure the encoder with the registry and options used for the operation.

## Usage

```go
raw, err := bsonbound.Marshal(doc, 16<<20, 1001, nil)
```

## Errors

ErrLimitExceeded covers byte, nesting and evaluator limits. Custom encoders must propagate writer errors. Use `errors.Is` for sentinel errors.
