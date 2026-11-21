# bsoncodec

```go
import "github.com/altessa-s/go-mongox/bsoncodec"
```

Build a MongoDB driver registry for optional values, exact decimals and civil dates. Register additional date types on the registry that will encode and
validate them.

## API

| Symbol                      | Purpose                                                       |
|-----------------------------|---------------------------------------------------------------|
| `NewRegistry`               | Build a registry with the shared codecs.                      |
| `RegisterDateType`          | Register an additional date type for encoding and validation. |
| `WithDateZeroAsEmptyString` | Encode a zero date as an empty string.                        |

## Usage

```go
reg := bsoncodec.NewRegistry()
```

## Zero dates

The default go-atlas-core civil date writes an empty string for zero. Other registered dates write null unless `WithDateZeroAsEmptyString` is set. Both
representations decode to zero.

## Errors

ErrNotADateType and ErrInvalidDate classify date failures. ErrDecimalOutOfRange covers unrepresentable numbers and coefficients over 4096 bits.
ErrDecimalNotFinite identifies NaN and infinity on read. Use `errors.Is` for sentinel errors.
