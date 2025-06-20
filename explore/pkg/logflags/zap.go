package logflags

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func HTTPLogger() Logger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:      "timestamp",
		LevelKey:     "level",
		MessageKey:   "message",
		EncodeLevel:  zapcore.CapitalLevelEncoder,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	level := zapcore.ErrorLevel
	if http {
		level = zapcore.DebugLevel
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(logOut)),
		level,
	)

	return zap.New(core, zap.AddCaller()).Sugar()
}
