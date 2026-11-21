# bsonfilter

```go
import "github.com/altessa-s/go-mongox/bsonfilter"
```

Prepare accepts bson.D, bson.M, map[string]any, bson.Raw and nil. Maps at every depth are sorted. Ordered documents, arrays and raw BSON retain their order. Use bson.D for an embedded literal whose field order matters. Nil means an empty document. A nil registry uses driver defaults. Validate also accepts individual admitted values. AdmitScalar registers a trusted struct codec for one registry. Its encoded output is checked.

## Usage

```go
raw, err := bsonfilter.Prepare(ctx, filter, reg, opts)
```

## Errors

ErrDangerousOperator, ErrValueType, ErrMalformedRaw, ErrTooDeep and ErrTooLarge classify rejected input. Writer limits also match bsonbound.ErrLimitExceeded. The walk budgets 1000 levels, 262144 visits and 16 MiB. Context cancellation is checked periodically. Use `errors.Is` for sentinel errors.
