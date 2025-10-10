package proxy

import (
	"fmt"
	"os"
	"strings"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

type InstanceLogger struct {
	InstanceID   string
	InstanceName string
	Port         string
	LogFilePath  string
	logger       *log.Entry
	fileLogger   *log.Logger
}

// NewInstanceLogger creates a logger with instance identification
func NewInstanceLogger(addr string, instanceName string) *InstanceLogger {
	return NewInstanceLoggerWithFile(addr, instanceName, "")
}

// NewInstanceLoggerWithFile creates a logger with instance identification and optional file output
func NewInstanceLoggerWithFile(addr string, instanceName string, logFilePath string) *InstanceLogger {
	// Extract port from address
	port := addr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		port = addr[idx+1:]
	}

	// Generate instance ID if name not provided
	if instanceName == "" {
		instanceName = fmt.Sprintf("proxy-%s", port)
	}

	il := &InstanceLogger{
		InstanceID:   uuid.NewV4().String()[:8],
		InstanceName: instanceName,
		Port:         port,
		LogFilePath:  logFilePath,
	}

	// Configure file logger if path provided
	if logFilePath != "" {
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.WithError(err).Errorf("Failed to open log file: %s", logFilePath)
		} else {
			// Create a dedicated logger for file output
			il.fileLogger = log.New()
			il.fileLogger.SetOutput(file)
			il.fileLogger.SetFormatter(&log.JSONFormatter{})
			
			// Use the file logger as base
			il.logger = il.fileLogger.WithFields(log.Fields{
				"instance_id":   il.InstanceID,
				"instance_name": il.InstanceName,
				"port":          il.Port,
			})
			return il
		}
	}

	// Default: use standard logger with persistent fields
	il.logger = log.WithFields(log.Fields{
		"instance_id":   il.InstanceID,
		"instance_name": il.InstanceName,
		"port":          il.Port,
	})

	return il
}

// WithFields adds additional fields to the logger
func (il *InstanceLogger) WithFields(fields log.Fields) *log.Entry {
	return il.logger.WithFields(fields)
}

// Info logs at info level
func (il *InstanceLogger) Info(args ...interface{}) {
	il.logger.Info(args...)
}

// Infof logs formatted at info level
func (il *InstanceLogger) Infof(format string, args ...interface{}) {
	il.logger.Infof(format, args...)
}

// Debug logs at debug level
func (il *InstanceLogger) Debug(args ...interface{}) {
	il.logger.Debug(args...)
}

// Debugf logs formatted at debug level
func (il *InstanceLogger) Debugf(format string, args ...interface{}) {
	il.logger.Debugf(format, args...)
}

// Error logs at error level
func (il *InstanceLogger) Error(args ...interface{}) {
	il.logger.Error(args...)
}

// Errorf logs formatted at error level
func (il *InstanceLogger) Errorf(format string, args ...interface{}) {
	il.logger.Errorf(format, args...)
}

// Warn logs at warn level
func (il *InstanceLogger) Warn(args ...interface{}) {
	il.logger.Warn(args...)
}

// Warnf logs formatted at warn level
func (il *InstanceLogger) Warnf(format string, args ...interface{}) {
	il.logger.Warnf(format, args...)
}

// Fatal logs at fatal level
func (il *InstanceLogger) Fatal(args ...interface{}) {
	il.logger.Fatal(args...)
}

// Fatalf logs formatted at fatal level
func (il *InstanceLogger) Fatalf(format string, args ...interface{}) {
	il.logger.Fatalf(format, args...)
}

// GetEntry returns the underlying logrus entry
func (il *InstanceLogger) GetEntry() *log.Entry {
	return il.logger
}
