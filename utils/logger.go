package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	DEBUG_ENABLED bool
	warningLogger *log.Logger
	infoLogger    *log.Logger
	errorLogger   *log.Logger
)

func OpenLogFile() error {
	now := time.Now()
	timestamp := now.Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_CLIENT_LOG", timestamp)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	infoLogger = log.New(f, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger = log.New(f, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(f, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	return nil
}

func Inform(v string, args ...any) {
	if DEBUG_ENABLED {
		infoLogger.Printf(v, args...)
	}
}

func WarnUser(v string, args ...any) {
	if DEBUG_ENABLED {
		warningLogger.Printf(v, args...)
	}
}

func ReportError(args ...any) {
	if DEBUG_ENABLED {
		errorLogger.Fatalln(args...)
	}
}
