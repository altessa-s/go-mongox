# bsonbound

```go
import "github.com/altessa-s/go-mongox/bsonbound"
```

Encode BSON with explicit limits on the number of bytes and open documents. Use the same registry and encoder options as the database operation.

## API

| Symbol           | Purpose                                                        |
|------------------|----------------------------------------------------------------|
| `Marshal`        | Encode a document within byte and depth limits.                |
| `NewValueWriter` | Wrap a value writer and optionally charge an evaluator budget. |
| `Bound`          | Inspect the encoded size and current nesting depth.            |

## Usage

```go
raw, err := bsonbound.Marshal(doc, 16<<20, 1001, nil)
```

## Accounting

A writer can charge an evaluator budget as it encodes. Its `Bound` reports the encoded size and current depth.

## Errors

ErrLimitExceeded covers byte, nesting and evaluator limits. Custom encoders must propagate writer errors. Use `errors.Is` for sentinel errors.
