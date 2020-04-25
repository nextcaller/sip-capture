package testhelpers

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// UpdateGolden provides the '-update' flag for `go test`
var updateGolden bool

func init() {
	flag.BoolVar(&updateGolden, "update", false, "update test cases")
}

// LoadGolden fetches the bytes previously saved with the name specified by
// the golden argument.
func LoadGolden(t *testing.T, goldfile string) []byte {
	t.Helper()
	expected, err := ioutil.ReadFile(filepath.Join("testdata", goldfile))
	if err != nil {
		t.Errorf("unable to load golden file %s, %v", goldfile, err)
	}
	return expected
}

// SaveGolden stores bytes to compare for future uses of LoadGolden.
func SaveGolden(t *testing.T, goldfile string, contents []byte) {
	t.Helper()
	if _, err := os.Stat("testdata"); os.IsNotExist(err) {
		if err := os.Mkdir("testdata", 0700); err != nil {
			t.Fatalf("unable to make testdata directory %v", err)
		}
	}

	fp := filepath.Join("testdata", goldfile)
	err := ioutil.WriteFile(fp, contents, 0600)
	if err != nil {
		t.Fatalf("unable to write golden file %s, %v", goldfile, err)
	}
}

// CompareGolden is shorthand for saving golden output if run with -update and
// then loading and comparing the golden against the given actual bytes.
// If -update flag was used, this should always return true.
func CompareGolden(t *testing.T, tname string, goldfile string, actual []byte) bool {
	t.Helper()

	if updateGolden {
		SaveGolden(t, goldfile, actual)
	}
	expected := LoadGolden(t, goldfile)
	if !bytes.Equal(actual, expected) {
		t.Errorf("%v: got: [%v] expecting: [%v]", tname, string(actual), string(expected))
		return false
	}
	return true
}
