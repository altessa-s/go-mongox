# go-mongox

[![CI](https://github.com/altessa-s/go-mongox/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/altessa-s/go-mongox/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/altessa-s/go-mongox.svg)](https://pkg.go.dev/github.com/altessa-s/go-mongox)
[![Go Report Card](https://goreportcard.com/badge/github.com/altessa-s/go-mongox)](https://goreportcard.com/report/github.com/altessa-s/go-mongox)
[![Release](https://img.shields.io/github/v/release/altessa-s/go-mongox)](https://github.com/altessa-s/go-mongox/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/altessa-s/go-mongox)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

```go
import "github.com/altessa-s/go-mongox/<package>"
```

MongoDB primitives shared by Go services and consumer adapters. Import individual packages for BSON encoding, filter preparation, signed keyset
pagination and index creation.

## Requirements

Go 1.27 or later and mongo-driver v2. CI exercises driver v2.8.0 and v2.10.1 against MongoDB 8.0.

## Installation

```sh
go get github.com/altessa-s/go-mongox@v0.1.0
```

## Packages

| Package                    | Description                                                               |
|----------------------------|---------------------------------------------------------------------------|
| [bsonbound](./bsonbound)   | Bounds BSON encoding by byte size, nesting depth and evaluator budget     |
| [bsoncodec](./bsoncodec)   | Registers BSON codecs for optional values, exact decimals and civil dates |
| [bsonfilter](./bsonfilter) | Validates filters and prepares ordered BSON for execution                 |
| [bsonraw](./bsonraw)       | Walks raw BSON and checks element boundaries                              |
| [indexes](./indexes)       | Resolves index names and creates indexes                                  |
| [keyset](./keyset)         | Reads signed keyset pages from MongoDB collections                        |

## Design principles

Validate filter values before encoding. Bind page tokens to the exact BSON used for the query. Ordered documents, scalar types and existing token
bindings are part of the compatibility contract. Consumers own collection lifecycle and index reconciliation.

## Versioning

The root module and its six packages share one semantic version. Historical BSON and token examples live in [testdata](./testdata).

## Development

| Command         | Description                                                                 |
|-----------------|-----------------------------------------------------------------------------|
| `make test`     | Run all tests                                                               |
| `make lint`     | Tidy, format and run golangci-lint with the repo config (`.golangci.yml`)   |
| `make generate` | Build the pinned `optgen` and golangci-lint into `bin/` and run go generate |
| `make bench`    | Run all benchmarks                                                          |

`make generate` builds [`optgen`](https://github.com/altessa-s/go-optgen) at the version pinned in `devtools/go.mod` into `bin/optgen` and runs
it from there; nothing is installed into `GOPATH` and an `optgen` on `PATH` is not used. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for every target.

Set `MONGO_URI` and run `make test-integration` to exercise a live MongoDB server. Each test creates and drops its own collection.

## License

[MIT](LICENSE)
