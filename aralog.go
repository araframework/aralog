package aralog

import (
	"runtime"
	"fmt"
)

func init() {
}

func Log(calldepth int, s string) {

	counter, file, line, ok := runtime.Caller(calldepth)

	fmt.Println("counter:", counter)
	fmt.Println("file:", file)
	fmt.Println("line:", line)
	fmt.Println("ok:", ok)

	fmt.Println("---")
}