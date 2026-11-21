// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package mongox_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Check the owning module as go-aip also hosts separately released consumer adapters.
func TestProductionDependencies(t *testing.T) {
	t.Parallel()

	const module = "github.com/altessa-s/go-mongox"

	edges := map[string][]string{
		"bsonbound":  nil,
		"bsoncodec":  {"bsonfilter"},
		"bsonfilter": {"bsonbound", "bsonraw"},
		"bsonraw":    nil,
		"indexes":    nil,
		"keyset":     {"bsonfilter", "bsonraw"},
	}

	type pkg struct {
		ImportPath string
		Standard   bool
		Module     *struct{ Path string }
		Imports    []string
	}

	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "-json=ImportPath,Standard,Module,Imports", "./...").Output()
	require.NoError(t, err)

	packages := map[string]pkg{}

	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p pkg

		require.NoError(t, dec.Decode(&p))
		packages[p.ImportPath] = p
	}

	for path, p := range packages {
		if p.Module == nil || p.Module.Path != module || (path == module && len(p.Imports) == 0) {
			continue
		}

		name := strings.TrimPrefix(path, module+"/")
		require.Contains(t, edges, name)

		for _, dep := range p.Imports {
			q := packages[dep]

			if q.Standard {
				continue
			}

			require.NotNil(t, q.Module, "%s imports %s", path, dep)

			switch q.Module.Path {
			case module:
				require.Contains(t, edges[name], strings.TrimPrefix(dep, module+"/"), "%s imports %s", path, dep)
			case "github.com/altessa-s/go-aip", "github.com/altessa-s/go-atlas-core", "github.com/shopspring/decimal", "go.mongodb.org/mongo-driver/v2":
			default:
				t.Errorf("%s imports %s from disallowed module %s", path, dep, q.Module.Path)
			}
		}
	}
}
