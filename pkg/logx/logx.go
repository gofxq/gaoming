package logx

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	sugar *zap.SugaredLogger
	base  *zap.Logger
}

func New(component string) *Logger {
	logDir := os.Getenv("GAOMING_LOG_DIR")
	if logDir == "" {
		logDir = "logs"
	}
	_ = os.MkdirAll(logDir, 0o755)

	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig())
	fileEncoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())

	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(logDir, component+".log"),
		MaxSize:    50,
		MaxBackups: 10,
		MaxAge:     3,
		Compress:   false,
	})

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zap.InfoLevel),
		zapcore.NewCore(fileEncoder, fileWriter, zap.InfoLevel),
	)

	base := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).With(zap.String("component", component))
	return &Logger{
		sugar: base.Sugar(),
		base:  base,
	}
}

func NewNop() *Logger {
	base := zap.NewNop()
	return &Logger{
		sugar: base.Sugar(),
		base:  base,
	}
}

func (l *Logger) Info(msg string, kv ...any) {
	l.sugar.Infow(msg, normalizeKeyvals(kv)...)
}

func (l *Logger) Warn(msg string, kv ...any) {
	l.sugar.Warnw(msg, normalizeKeyvals(kv)...)
}

func (l *Logger) Error(msg string, kv ...any) {
	l.sugar.Errorw(msg, normalizeKeyvals(kv)...)
}

func (l *Logger) Sync() error {
	if l == nil || l.base == nil {
		return nil
	}
	return l.base.Sync()
}

func normalizeKeyvals(kv []any) []any {
	if len(kv)%2 == 0 {
		return kv
	}
	out := make([]any, 0, len(kv)+1)
	out = append(out, kv...)
	out = append(out, "ignored", fmt.Sprint(kv[len(kv)-1]))
	return out
}
