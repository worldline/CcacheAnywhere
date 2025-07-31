package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
	// /home/rocky/repos/py_server_script/daemons
	filename := fmt.Sprintf("/home/rocky/repos/py_server_script/daemons/%s_CLIENT_LOG", timestamp)
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
	if DEBUG_ENABLED {
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
	if DEBUG_ENABLED {
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
	if DEBUG_ENABLED {
		errorLogger.Fatalln(args...)
	}
}
