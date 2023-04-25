package logging

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var _ log.Logger = (*ZapLogger)(nil)

type ZapLogger struct {
	log  *zap.Logger
	Sync func() error
	pool *sync.Pool
}

func NewLogger(logName string, env string, caller ...int) (log.Logger, func()) {
	zapLogger := NewZapLogger(logName, env, caller...)
	return log.With(
			zapLogger,
		), func() {
			zapLogger.Sync()
		}
}

func NewZapLogger(logName string, env string, caller ...int) *ZapLogger {
	encoder := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stack",
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	var level zap.AtomicLevel
	if env == "dev" {
		level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	var skipCaller = 3
	if len(caller) == 1 {
		skipCaller = caller[0]
	}
	writeSyncer := getLogWriter(logName)
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoder),
		zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(os.Stdout), &zapcore.BufferedWriteSyncer{WS: writeSyncer},
		), level)
	zapLogger := zap.New(core, zap.AddStacktrace(
		zap.NewAtomicLevelAt(zapcore.ErrorLevel)),
		zap.AddCaller(),
		zap.AddCallerSkip(skipCaller),
	)
	return &ZapLogger{log: zapLogger, Sync: zapLogger.Sync, pool: &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}}
}

func (l *ZapLogger) Log(level log.Level, keyvals ...interface{}) error {
	if len(keyvals) == 0 || len(keyvals)%2 != 0 {
		l.log.Warn(fmt.Sprint("Keyvalues must appear in pairs: ", keyvals))
		return nil
	}
	var data []zap.Field
	buf := l.pool.Get().(*bytes.Buffer)
	var msg string
	for i := 0; i < len(keyvals); i += 2 {
		switch keyvals[i].(string) {
		case "msg":
			msg = keyvals[i+1].(string)
		default:
			v, ok := keyvals[i+1].(string)
			if ok && len(v) == 0 {
				continue
			}
			fmt.Fprintf(buf, " %s=%v", keyvals[i], keyvals[i+1])
		}
	}
	s := fmt.Sprintf("%v %v", buf.String(), msg)
	buf.Reset()
	l.pool.Put(buf)
	switch level {
	case log.LevelDebug:
		l.log.Debug(s, data...)
	case log.LevelInfo:
		l.log.Info(s, data...)
	case log.LevelWarn:
		l.log.Warn(s, data...)
	case log.LevelError:
		l.log.Error(s, data...)
	}
	return nil
}

func getLogWriter(logName string) zapcore.WriteSyncer {
	if len(logName) == 0 {
		logName = "serviceLog"
	}
	lumberJackLogger := &lumberjack.Logger{
		Filename:   fmt.Sprintf("%s.log", logName),
		MaxSize:    100,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   false,
	}
	return zapcore.AddSync(lumberJackLogger)
}
