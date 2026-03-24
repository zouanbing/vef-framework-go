package logx

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/coldsmirk/vef-framework-go/logx"
)

type zapLogger struct {
	logger *zap.SugaredLogger
}

func (l *zapLogger) Named(name string) logx.Logger {
	return &zapLogger{
		logger: l.logger.Named(name),
	}
}

func (l *zapLogger) WithCallerSkip(skip int) logx.Logger {
	return &zapLogger{
		logger: l.logger.WithOptions(zap.AddCallerSkip(skip)),
	}
}

func (l *zapLogger) Enabled(level logx.Level) bool {
	zapLevel := toZapLevel(level)

	return l.logger.Level().Enabled(zapLevel)
}

func toZapLevel(level logx.Level) zapcore.Level {
	switch level {
	case logx.LevelDebug:
		return zap.DebugLevel
	case logx.LevelWarn:
		return zap.WarnLevel
	case logx.LevelError:
		return zap.ErrorLevel
	case logx.LevelPanic:
		return zap.PanicLevel
	case logx.LevelInfo:
		fallthrough
	default:
		return zap.InfoLevel
	}
}

func (l *zapLogger) Sync() {
	if err := l.logger.Sync(); err != nil {
		l.Errorf("error occurred while flushing logger: %v", err)
	}
}

func (l *zapLogger) Debug(message string) {
	l.logger.Debug(message)
}

func (l *zapLogger) Debugf(template string, args ...any) {
	l.logger.Debugf(template, args...)
}

func (l *zapLogger) Info(message string) {
	l.logger.Info(message)
}

func (l *zapLogger) Infof(template string, args ...any) {
	l.logger.Infof(template, args...)
}

func (l *zapLogger) Warn(message string) {
	l.logger.Warn(message)
}

func (l *zapLogger) Warnf(template string, args ...any) {
	l.logger.Warnf(template, args...)
}

func (l *zapLogger) Error(message string) {
	l.logger.Error(message)
}

func (l *zapLogger) Errorf(template string, args ...any) {
	l.logger.Errorf(template, args...)
}

func (l *zapLogger) Panic(message string) {
	l.logger.Panic(message)
}

func (l *zapLogger) Panicf(template string, args ...any) {
	l.logger.Panicf(template, args...)
}
