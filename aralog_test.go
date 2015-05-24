package aralog

import (
	"testing"
	"github.com/araframework/aralog"
)

func TestAraLog(t *testing.T) {
	logger, err := aralog.NewFileLogger("ara.log", aralog.Llongfile | aralog.Ltime)
	if err != nil {
		t.Error("new logger error: ", err)
	}

	logger.Debug("log a test string")
}