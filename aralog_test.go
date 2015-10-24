package aralog

import (
	"testing"
)

func TestAraLog(t *testing.T) {
	logger, err := NewFileLogger("ara.log", Llongfile | Ltime)
	if err != nil {
		t.Error("new logger error: ", err)
	}

	logger.Debug("log a test string")
}
