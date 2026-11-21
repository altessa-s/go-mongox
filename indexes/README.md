# indexes

```go
import "github.com/altessa-s/go-mongox/indexes"
```

Names resolves explicit and compound index names in model order. Ensure checks every name before its first server command and creates indexes using private option builders. An empty model list succeeds. Conflicting indexes are left in place.

## Usage

```go
err := indexes.Ensure(ctx, coll, models)
```

## Errors

ErrInvalidIndexModels identifies invalid or duplicate names and invalid key models. Inspect server conflicts with errors.As and mongo.ServerError.HasErrorCode. Use `errors.Is` for sentinel errors.
