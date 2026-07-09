package utils

import (
	"fmt"
	"os"
	"time"
)

const logFile = "./logs/app.log"

func Debugf(format string, args ...any) {
	if os.Getenv("CTRWATCH_DEBUG") == "" {
		return
	}
	if err := os.MkdirAll("./logs", 0755); err != nil {
		return
	}
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	_, _ = fmt.Fprintf(file, "%s "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}
