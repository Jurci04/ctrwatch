// ponytail: kept for debugging even though no code imports it yet.
// Remove this package once ctrwatch has a --debug flag or structured logging.
package utils

import (
	"fmt"
	"os"
)

var LOG_FILE_DIR = "./logs/app.log"

func LogToFile(message string) {
	file, err := os.OpenFile(LOG_FILE_DIR, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening log file")
		return
	}
	defer func() { _ = file.Close() }()
	_, _ = file.WriteString(message)
}

func init() {
	_ = os.MkdirAll("./logs", 0755)
	file, err := os.Create(LOG_FILE_DIR)
	if err == nil {
		_ = file.Close()
	}
}
