package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

// New инициализирует и настраивает логгер slog.
// Он принимает уровень логирования и окружение.
func New(level string, env string) *slog.Logger {
	var logLevel slog.Level

	// 1. Парсим уровень логирования
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		// "info" и всё остальное
		logLevel = slog.LevelInfo
	}

	// 2. Выбираем формат логов: JSON для продакшена, текст — для local/dev
	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true, // добавляет file:line в лог
	}

	var handler slog.Handler
	switch env {
	case envProd:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	// 3. Создаём логгер
	logger := slog.New(handler)

	// 4. Делаем его логгером по умолчанию
	slog.SetDefault(logger)

	return logger
}

// NewContextWithLogger добавляет логгер в контекст.
// Полезно, когда нужно прокидывать логгер глубоко в цепочку вызовов.
func NewContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	type loggerKey struct{}
	return context.WithValue(ctx, loggerKey{}, logger)
}

// LoggerFromContext извлекает логгер из контекста.
// Если его нет — возвращает slog.Default().
func LoggerFromContext(ctx context.Context) *slog.Logger {
	type loggerKey struct{}

	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return logger
	}

	// Возвращаем default логгер
	return slog.Default()
}
