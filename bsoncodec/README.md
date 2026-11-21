# bsoncodec

```go
import "github.com/altessa-s/go-mongox/bsoncodec"
```

NewRegistry returns a fresh driver registry. RegisterDateType installs a codec and admits its type for filters using that registry. The default atlas-core civil date writes an empty string for zero. Other registered dates write null unless WithDateZeroAsEmptyString is set. Both representations decode to zero.

## Usage

```go
reg := bsoncodec.NewRegistry()
```

## Errors

ErrNotADateType and ErrInvalidDate classify date failures. ErrDecimalOutOfRange covers unrepresentable numbers and coefficients over 4096 bits. ErrDecimalNotFinite identifies NaN and infinity on read. Use `errors.Is` for sentinel errors.
