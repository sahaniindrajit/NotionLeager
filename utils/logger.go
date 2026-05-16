package utils

import (
	"io"
	"log"
	"os"
)

var debugLogger = log.New(io.Discard, "", 0)

// InitLogger enables debug logging when env is not "production".
func InitLogger(env string) {
	if env == "production" {
		debugLogger = log.New(io.Discard, "", 0)
		return
	}
	debugLogger = log.New(os.Stderr, "", log.LstdFlags)
}

func Debugln(v ...any)                 { debugLogger.Println(v...) }
func Debugf(format string, v ...any)   { debugLogger.Printf(format, v...) }
