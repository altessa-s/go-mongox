# keyset

```go
import "github.com/altessa-s/go-mongox/keyset"
```

FindPage binds the sort, prepared BSON filter, subject and timestamp precision to each continuation. Set Query.Registry and Query.BSONOptions to the effective encoding settings. The collection decodes results. Supply a stable unique TiebreakPath (default _id) and keep all sort fields in the projection. With a nil signer a single page can be read.

## Usage

```go
page, err := keyset.FindPage[Widget](ctx, coll, signer, keyset.Query{Limit: 50})
```

## Errors

ErrSort, ErrProjection and ErrMissingKey classify unsupported paging inputs. ErrTokensRequired wraps pagetoken.ErrCodecRequired. Resolve failures preserve pagetoken errors. DecodeError unwraps the document decoder failure. Use `errors.Is` for sentinel errors.
