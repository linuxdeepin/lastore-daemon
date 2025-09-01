// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

type Provider interface {
	Get(key string) interface{}
	GetBool(key string) bool
	// GetDuration(key string) time.Duration
	// GetFloat64(key string) float64
	// GetInt(key string) int
	// GetInt64(key string) int64
	// GetSizeInBytes(key string) uint
	// GetString(key string) string
	// GetStringMap(key string) map[string]interface{}
	// GetStringMapString(key string) map[string]string
	// GetStringMapStringSlice(key string) map[string][]string
	// GetStringSlice(key string) []string
	// GetTime(key string) time.Time
	// InConfig(key string) bool
	// IsSet(key string) bool
	GetJsonFormat() bool
	GetLogLevel() string
}
type LoggerConfig struct {
	jsonFormat bool   `json:"json_logs,omitempty"`
	debugMode  bool   `json:"debug"`
	logLevel   string `json:"log_level"`
}

func (config *LoggerConfig) Get(k string) interface{} {
	return "debug"
}

func (config *LoggerConfig) GetBool(k string) bool {
	return false
}

// func (config *LoggerConfig) GetDuration(k string)  time.Duration {
// 	return time.Duration
// }

func (config *LoggerConfig) GetJsonFormat() bool {
	return false
}

func (config *LoggerConfig) GetLogLevel() string {
	getDebug := os.Getenv("DEEPIN_SYSTEM_UPDATE_TOOLS_DEBUG")
	if getDebug != "" {
		return "debug"
	} else {
		// FIXME(heysion) return "info"
		return "info"
		// return "debug"
	}
}

var (
	defaultLoggerConfig *LoggerConfig
	defaultLogger       *logrus.Logger
)

// Logger defines a set of methods for writing application logs. Derived from and
// inspired by logrus.Entry.
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Debugln(args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Errorln(args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Fatalln(args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Infoln(args ...interface{})
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	Panicln(args ...interface{})
	Print(args ...interface{})
	Printf(format string, args ...interface{})
	Println(args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
	Warningln(args ...interface{})
	Warnln(args ...interface{})
}

func LogConfig() Provider {
	return defaultLoggerConfig
}

func init() {
	defaultLogger = newLogrusLogger(LogConfig())
}

// NewLogger returns a configured logrus instance
func NewLogger(cfg Provider) *logrus.Logger {
	return newLogrusLogger(cfg)
}

func newLogrusLogger(cfg Provider) *logrus.Logger {
	l := logrus.New()

	if cfg.GetJsonFormat() {
		l.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		l.SetFormatter(&logrus.TextFormatter{
			ForceColors: true, //show colors
			//DisableColors:true,//remove colors
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	l.SetOutput(os.Stdout)

	switch cfg.GetLogLevel() {
	case "debug":
		l.Level = logrus.DebugLevel
	case "warning":
		l.Level = logrus.WarnLevel
	case "info":
		l.Level = logrus.InfoLevel
	default:
		l.Level = logrus.DebugLevel
	}

	return l
}

func SetDebugEnabled() {
	if defaultLogger != nil {
		defaultLogger.Level = logrus.DebugLevel
	}
}

// Fields is a map string interface to define fields in the structured log
type Fields map[string]interface{}

// With allow us to define fields in out structured logs
func (f Fields) With(k string, v interface{}) Fields {
	f[k] = v
	return f
}

// WithFields allow us to define fields in out structured logs
func (f Fields) WithFields(f2 Fields) Fields {
	for k, v := range f2 {
		f[k] = v
	}
	return f
}

// WithFields allow us to define fields in out structured logs
func WithFields(fields Fields) Logger {
	return defaultLogger.WithFields(logrus.Fields(fields))
}

// Debug package-level convenience method.
func Debug(args ...interface{}) {
	defaultLogger.Debug(args...)
}

// Debugf package-level convenience method.
func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

// Debugln package-level convenience method.
func Debugln(args ...interface{}) {
	defaultLogger.Debugln(args...)
}

// Error package-level convenience method.
func Error(args ...interface{}) {
	defaultLogger.Error(args...)
}

// Errorf package-level convenience method.
func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

// Errorln package-level convenience method.
func Errorln(args ...interface{}) {
	defaultLogger.Errorln(args...)
}

// Fatal package-level convenience method.
func Fatal(args ...interface{}) {
	defaultLogger.Fatal(args...)
}

// Fatalf package-level convenience method.
func Fatalf(format string, args ...interface{}) {
	defaultLogger.Fatalf(format, args...)
}

// Fatalln package-level convenience method.
func Fatalln(args ...interface{}) {
	defaultLogger.Fatalln(args...)
}

// Info package-level convenience method.
func Info(args ...interface{}) {
	defaultLogger.Info(args...)
}

// Infof package-level convenience method.
func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

// Infoln package-level convenience method.
func Infoln(args ...interface{}) {
	defaultLogger.Infoln(args...)
}

// Panic package-level convenience method.
func Panic(args ...interface{}) {
	defaultLogger.Panic(args...)
}

// Panicf package-level convenience method.
func Panicf(format string, args ...interface{}) {
	defaultLogger.Panicf(format, args...)
}

// Panicln package-level convenience method.
func Panicln(args ...interface{}) {
	defaultLogger.Panicln(args...)
}

// Print package-level convenience method.
func Print(args ...interface{}) {
	defaultLogger.Print(args...)
}

// Printf package-level convenience method.
func Printf(format string, args ...interface{}) {
	defaultLogger.Printf(format, args...)
}

// Println package-level convenience method.
func Println(args ...interface{}) {
	defaultLogger.Println(args...)
}

// Warn package-level convenience method.
func Warn(args ...interface{}) {
	defaultLogger.Warn(args...)
}

// Warnf package-level convenience method.
func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

// Warning package-level convenience method.
func Warning(args ...interface{}) {
	defaultLogger.Warning(args...)
}

// Warningf package-level convenience method.
func Warningf(format string, args ...interface{}) {
	defaultLogger.Warningf(format, args...)
}

// Warningln package-level convenience method.
func Warningln(args ...interface{}) {
	defaultLogger.Warningln(args...)
}

// Warnln package-level convenience method.
func Warnln(args ...interface{}) {
	defaultLogger.Warnln(args...)
}
