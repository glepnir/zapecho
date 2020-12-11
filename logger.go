package logger

import (
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewDevelopmentEncoderConfig returns an opinionated zapcore.EncoderConfig for
// development environments.
func NewDevelopmentEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		MessageKey:     "M",
		LevelKey:       "L",
		TimeKey:        "T",
		NameKey:        "N",
		CallerKey:      "C",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: nil,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// NewDevelopmentConfig returns a reasonable development logging configuration.
func NewDevelopmentConfig(lv zap.AtomicLevel) zap.Config {
	cfg := zap.Config{
		Level:             lv,
		Development:       true,
		DisableCaller:     true,
		DisableStacktrace: false,
		Encoding:          "console",
		EncoderConfig:     NewDevelopmentEncoderConfig(),
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
	}

	return cfg
}

// NewLogger returns the new zap.Logger with concurrency-safe SyncBuffer.
func NewLogger(lv zap.AtomicLevel, opts ...zap.Option) *zap.Logger {
	c := zap.NewProductionConfig()
	c.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	if lv.Level().Enabled(zapcore.DebugLevel) {
		c = NewDevelopmentConfig(lv)
	}

	logger, err := c.Build(opts...)
	if err != nil {
		panic(fmt.Errorf("logging.NewLogger: %v", err))
	}

	return logger
}

func ZapMiddleware(atom zap.AtomicLevel) echo.MiddlewareFunc {

	middlewareLogger := NewLogger(atom)

	defer middlewareLogger.Sync()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()

			fields := []zapcore.Field{
				zap.String("remote_ip", c.RealIP()),
				zap.String("latency", time.Since(start).String()),
				zap.String("host", req.Host),
				zap.String("request", fmt.Sprintf("%s %s", req.Method, req.RequestURI)),
				zap.Int("status", res.Status),
				zap.Int64("size", res.Size),
				zap.String("user_agent", req.UserAgent()),
			}

			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
				fields = append(fields, zap.String("request_id", id))
			}

			n := res.Status
			switch {
			case n >= 500:
				middlewareLogger.Error("Server error", fields...)
			case n >= 400:
				middlewareLogger.Warn("Client error", fields...)
			case n >= 300:
				middlewareLogger.Info("Redirection", fields...)
			default:
				middlewareLogger.Info("Success", fields...)
			}

			return nil
		}
	}
}
