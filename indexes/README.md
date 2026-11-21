# indexes

```go
import "github.com/altessa-s/go-mongox/indexes"
```

Resolve explicit and compound index names in model order, or create the requested indexes on a collection.

## API

| Symbol   | Purpose                                                  |
|----------|----------------------------------------------------------|
| `Names`  | Resolve explicit and derived index names in model order. |
| `Ensure` | Validate names and create the requested indexes.         |

## Usage

```go
err := indexes.Ensure(ctx, coll, models)
```

## Creation

`Ensure` checks every name before its first server command and creates indexes using private option builders. An empty model list succeeds. Conflicting
indexes are left in place.

## Errors

ErrInvalidIndexModels identifies invalid or duplicate names and invalid key models. Inspect server conflicts with errors.As and
mongo.ServerError.HasErrorCode. Use `errors.Is` for sentinel errors.
