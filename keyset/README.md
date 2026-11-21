# keyset

```go
import "github.com/altessa-s/go-mongox/keyset"
```

Read a MongoDB collection one page at a time. Each signed continuation binds the sort, prepared BSON filter, subject and timestamp precision.

## API

| Symbol        | Purpose                                                        |
|---------------|----------------------------------------------------------------|
| `Query`       | Specify the filter, sort, continuation and encoding options.   |
| `FindPage`    | Read a page and sign the continuation when more results exist. |
| `DecodeError` | Wrap a failure decoding a result document.                     |

## Usage

```go
page, err := keyset.FindPage[Widget](ctx, coll, signer, keyset.Query{Limit: 50})
```

## Query settings

Set `Query.Registry` and `Query.BSONOptions` to the effective encoding settings. The collection decodes results. Supply a stable unique `TiebreakPath`
(default `_id`) and keep all sort fields in the projection: a projection that drops one fails with ErrProjection before the query. With a
nil signer a single page can be read.

Every document needs a scalar of one BSON type in each sort field. A document whose sort field is null or missing is never skipped: it sorts
first in an ascending and last in a descending sort, and a page that ends on it fails with ErrMissingKey.

## Errors

ErrSort, ErrProjection, ErrPrecision and ErrMissingKey classify unsupported paging inputs. ErrTokensRequired wraps pagetoken.ErrCodecRequired. Resolve failures
preserve pagetoken errors. DecodeError unwraps the document decoder failure. Use `errors.Is` for sentinel errors.
