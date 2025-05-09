package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *zap.Logger

func InitLogger() {
	// 定义日志级别过滤器
	debugPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.DebugLevel
	})

	infoPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.InfoLevel
	})

	warnPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == zapcore.WarnLevel
	})

	errorPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	// 日志轮转配置
	debugWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/debug.log",
		MaxSize:    1024, // MB
		MaxBackups: 10,
		MaxAge:     365, // days
		Compress:   true,
	})

	infoWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/info.log",
		MaxSize:    1024,
		MaxBackups: 10,
		MaxAge:     365,
		Compress:   true,
	})

	warnWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/warn.log",
		MaxSize:    1024,
		MaxBackups: 10,
		MaxAge:     365,
		Compress:   true,
	})

	errorWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/error.log",
		MaxSize:    1024,
		MaxBackups: 10,
		MaxAge:     365,
		Compress:   true,
	})

	// 编码器配置
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// 多输出核心
	core := zapcore.NewTee(
		// Debug 日志只写入 debug.log
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(debugWriter),
			debugPriority,
		),
		// Info 日志写入 info.log
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(infoWriter),
			infoPriority,
		),
		// Warn 日志写入 warn.log
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(warnWriter),
			warnPriority,
		),
		// Error 及以上日志写入 error.log
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(errorWriter),
			errorPriority,
		),
		// 控制台输出所有级别日志
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			zapcore.DebugLevel,
		),
	)

	Logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

func Sync() {
	_ = Logger.Sync()
}
