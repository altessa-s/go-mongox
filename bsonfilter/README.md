# bsonfilter

```go
import "github.com/altessa-s/go-mongox/bsonfilter"
```

Validate filter values and prepare BSON for execution. Pass the prepared bytes to the collection with the registry and BSON options used during
preparation.

## API

| Symbol        | Purpose                                              |
|---------------|------------------------------------------------------|
| `Validate`    | Check whether a Go value can appear in a filter.     |
| `Prepare`     | Validate and encode a complete filter for execution. |
| `AdmitScalar` | Admit a trusted scalar type for a specific registry. |

## Usage

```go
raw, err := bsonfilter.Prepare(ctx, filter, reg, opts)
if err != nil {
    return err
}
result := collection.FindOne(ctx, raw)
```

## Ordering and inputs

`Prepare` accepts `bson.D`, `bson.M`, `map[string]any`, `bson.Raw` and nil. Maps at every depth are sorted. Ordered documents, arrays and raw BSON
retain their order. Use `bson.D` for an embedded literal whose field order matters. Nil means an empty document. A nil registry uses driver defaults. A
trusted scalar registered with `AdmitScalar` also has its encoded output checked.

## Errors

ErrDangerousOperator, ErrValueType, ErrMalformedRaw, ErrTooDeep and ErrTooLarge classify rejected input. Writer limits also match
bsonbound.ErrLimitExceeded. The walk budgets 1000 levels, 262144 visits and 16 MiB. Context cancellation is checked periodically. Use `errors.Is` for
sentinel errors.
