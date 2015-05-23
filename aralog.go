package aralog

import (
	"runtime"
	"fmt"
	"io"
	"sync"
)

type Logger struct {
	mutex sync.Mutex // ensures atomic writes; protects the following fields
	prefix string     // prefix to write at beginning of each line
	flag int        // properties
	out io.Writer  // destination for output
	buf []byte     // for accumulating text to write
}

func init() {
}

func Log(callDepth int, s string) {

	_, file, line, ok := runtime.Caller(callDepth)
	if !ok {
		file = "???"
		line = 0
	}

	fmt.Println("file:", file)
	fmt.Println("line:", line)
	fmt.Println("ok:", ok)

	fmt.Println("---")
}