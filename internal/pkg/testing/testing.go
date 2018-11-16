package testing

import (
	"os"
	"path"
	"runtime"
)

// make sure we hop to the project root when imported. This is to make life easier for tests so they can include files from testdata
// without needing to know its relative location in the tree
func init() {
	_, filename, _, _ := runtime.Caller(0)
	// hop back 2 directories, expecting this is internal/pkg/testing
	d := path.Join(path.Dir(filename), "../../..")
	err := os.Chdir(d)
	if err != nil {
		panic(err)
	}
}
