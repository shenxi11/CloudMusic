package logger

import (
	"io"
	"log"
	"os"
)

var logger *log.Logger

// Init 初始化日志
func Init(logFile string) error {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	mw := io.MultiWriter(os.Stdout, file)
	logger = log.New(mw, "", log.LstdFlags|log.Lshortfile)
	log.SetOutput(mw)

	return nil
}

// Info 信息日志
func Info(format string, v ...interface{}) {
	if logger != nil {
		logger.Printf("[INFO] "+format, v...)
	} else {
		log.Printf("[INFO] "+format, v...)
	}
}

// Error 错误日志
func Error(format string, v ...interface{}) {
	if logger != nil {
		logger.Printf("[ERROR] "+format, v...)
	} else {
		log.Printf("[ERROR] "+format, v...)
	}
}

// Warn 警告日志
func Warn(format string, v ...interface{}) {
	if logger != nil {
		logger.Printf("[WARN] "+format, v...)
	} else {
		log.Printf("[WARN] "+format, v...)
	}
}

// Debug 调试日志
func Debug(format string, v ...interface{}) {
	if logger != nil {
		logger.Printf("[DEBUG] "+format, v...)
	} else {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Fatal 致命错误日志
func Fatal(format string, v ...interface{}) {
	if logger != nil {
		logger.Fatalf("[FATAL] "+format, v...)
	} else {
		log.Fatalf("[FATAL] "+format, v...)
	}
}
