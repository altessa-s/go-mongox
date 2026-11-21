# bsonraw

```go
import "github.com/altessa-s/go-mongox/bsonraw"
```

Walk a BSON value without recursion and check its lengths and terminators. A visitor can inspect each element or return an error to stop the walk.

## API

| Symbol  | Purpose                                             |
|---------|-----------------------------------------------------|
| `Walk`  | Validate the input and visit embedded elements.     |
| `Visit` | Receive the depth, BSON type and key of an element. |

## Usage

```go
err := bsonraw.Walk(bson.TypeEmbeddedDocument, raw, 1000, nil)
```

## Visitor

The visitor receives the depth, type and a key slice that aliases the input.

## Errors

ErrMalformed identifies invalid BSON structure. ErrTooDeep identifies the caller’s nesting limit. Visitor errors propagate unchanged. Use `errors.Is`
for sentinel errors.
