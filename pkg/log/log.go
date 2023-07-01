package log

import (
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	logger         *log.Logger
	initOnce       sync.Once
	DefaultLogFile = "bvcni.log"
)

// InitLogger initializes the logger configuration
func InitLogger(logFile string) {
	initOnce.Do(func() {
		if logFile == "" {
			logFile = DefaultLogFile // default log file path
			fmt.Println("logFile is empty, using default path:", logFile)
		}

		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		logger = log.New(file, "", log.LstdFlags)
	})
}

func Debugf(template string, args ...interface{}) {
	logger.Printf(template, args...)
}
