// ! This package is only used when the debug flag is enabled
// otherwise the log file will not be created!
package logger

import (
	"ccache-backend-client/internal/constants"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	warningLogger *log.Logger
	infoLogger    *log.Logger
	errorLogger   *log.Logger
)

func OpenLogFile() error {
	now := time.Now()
	timestamp := now.Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_CLIENT_LOG", timestamp)
	fmt.Println("Helper logs on", filename)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	infoLogger = log.New(f, "INFO: ", log.Ldate|log.Ltime)
	warningLogger = log.New(f, "WARNING: ", log.Ldate|log.Ltime)
	errorLogger = log.New(f, "ERROR: ", log.Ldate|log.Ltime)
	return nil
}

func LOG(v string, args ...any) {
	if constants.DEBUG_ENABLED {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			infoLogger.Printf(v, args...)
			return
		}

		filename := filepath.Base(file)
		message := fmt.Sprintf(v, args...)
		infoLogger.Printf("%s:%d: %s\n", filename, line, message)
	}
}

func WARN(v string, args ...any) {
	if constants.DEBUG_ENABLED {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			infoLogger.Printf(v, args...)
			return
		}

		filename := filepath.Base(file)
		message := fmt.Sprintf(v, args...)
		warningLogger.Printf("%s:%d: %s\n", filename, line, message)
	}
}

func TERM(args ...any) {
	if constants.DEBUG_ENABLED {
		errorLogger.Fatalln(args...)
	}
}
