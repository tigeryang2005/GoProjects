package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *zap.Logger

func InitLogger() {
	// 日志级别
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel && lvl >= zapcore.DebugLevel
	})

	// 日志轮转配置
	infoWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/info.log",
		MaxSize:    200, // MB
		MaxBackups: 10,
		MaxAge:     365, // days
		Compress:   true,
	})

	debugWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/debug.log",
		MaxSize:    200, // MB
		MaxBackups: 10,
		MaxAge:     365, // days
		Compress:   true,
	})

	errorWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/error.log",
		MaxSize:    200,
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
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(debugWriter),
			lowPriority,
		),
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(infoWriter),
			lowPriority,
		),
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(errorWriter),
			highPriority,
		),
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
