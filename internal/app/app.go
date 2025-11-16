package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	gin "github.com/gin-gonic/gin"

	"github.com/ItsXomyak/restaurant-menu-parser/config"
	httpAPI "github.com/ItsXomyak/restaurant-menu-parser/internal/adapters/http"
	"github.com/ItsXomyak/restaurant-menu-parser/internal/adapters/parser"
	"github.com/ItsXomyak/restaurant-menu-parser/internal/adapters/repository"
	"github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
	"github.com/ItsXomyak/restaurant-menu-parser/internal/usecase"
	"github.com/ItsXomyak/restaurant-menu-parser/pkg/logger"
	"github.com/ItsXomyak/restaurant-menu-parser/pkg/rabbit"
	"github.com/ItsXomyak/restaurant-menu-parser/pkg/trm"
)

type App struct {
	cfg          config.Config
	logger       *slog.Logger
	mongoClient  *mongo.Client
	rabbitClient *rabbit.RabbitMQ
	httpServer   *http.Server

    parsingUseCase *usecase.ParsingUseCase
    productUseCase *usecase.ProductUseCase
}

func New(ctx context.Context) (*App, error) {
	cfg, innerErr := config.New()
	if innerErr != nil {
		return nil, fmt.Errorf("ошибка инициализации конфига: %w", innerErr)
	}

	log := logger.New(cfg.Logger.Level, cfg.Env)
	log.Info("Логгер инициализирован", "level", cfg.Logger.Level, "env", cfg.Env)

	mongoOptions := options.Client().ApplyURI(cfg.Mongo.URI)
	mongoOptions.SetMaxPoolSize(100)
	mongoOptions.SetMinPoolSize(10)

	var mongoClient *mongo.Client
	var err error
	maxRetries := 5

	for i := 0; i < maxRetries; i++ {
		log.Warn("Попытка подключения к MongoDB...", slog.Int("попытка", i+1))
		
		// Контекст для каждой попытки подключения (5 секунд)
		connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		
		// ШАГ А: Создаем клиент
		mongoClient, err = mongo.Connect(connectCtx, mongoOptions)
		if err != nil {
			cancel() // Закрываем контекст, если Connect упал (ошибка DNS/URI)
			log.Warn("Ошибка создания клиента Mongo", slog.Any("error", err))
			time.Sleep(3 * time.Second)
			continue
		}

		// ШАГ Б: Пингуем, чтобы проверить, что сервер готов
		if err = mongoClient.Ping(connectCtx, nil); err != nil {
			cancel() // Закрываем контекст, если Ping упал (ошибка таймаута/сервер не готов)
			log.Warn("Ошибка пинга Mongo", slog.Any("error", err))
			time.Sleep(3 * time.Second)
			continue
		}

		// Если оба шага прошли - все ОК
		cancel() // Успех, закрываем контекст и выходим из цикла
		err = nil 
		break
	}

	if err != nil {
		log.Error("Не удалось подключиться к MongoDB после нескольких попыток", slog.Any("error", err))
		return nil, err
	}
	log.Info("MongoDB успешно подключен")
	mongoDB := mongoClient.Database(cfg.Mongo.Name)

	rabbitConnectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rabbitClient, err := rabbit.New(rabbitConnectCtx, cfg.Rabbit.URI)
	if err != nil {
		log.Error("Не удалось подключиться к RabbitMQ", slog.Any("error", err))
		return nil, err
	}

	trmManager := trm.New(mongoClient)
	dbHealth := repository.NewMongoHealthChecker(mongoClient)

	taskRepo, err := repository.NewMongoTaskRepository(mongoDB)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания TaskRepository: %w", err)
	}
	menuRepo, err := repository.NewMongoMenuRepository(mongoDB)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания MenuRepository: %w", err)
	}
	auditRepo, err := repository.NewMongoAuditRepository(mongoDB)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания AuditRepository: %w", err)
	}

	saJSON, err := base64.StdEncoding.DecodeString(cfg.Google.ServiceAccountJSON)
	if err != nil {
		return nil, fmt.Errorf("ошибка декодирования Google Service Account JSON (Base64): %w", err)
	}
	parserCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	gParser, err := parser.NewGoogleSheetParser(parserCtx, saJSON)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации Google Parser: %w", err)
	}

    healthUC := usecase.NewHealthUseCase(dbHealth, rabbitClient)
    parsingUC := usecase.NewParsingUseCase(taskRepo, rabbitClient, trmManager, gParser, menuRepo)
    productUC := usecase.NewProductUseCase(rabbitClient, menuRepo, auditRepo, trmManager)

    if cfg.Env == "prod" {
        gin.SetMode(gin.ReleaseMode)
    }
	router := gin.Default()

	taskHandler := httpAPI.NewTaskHandler(parsingUC, log)
	productHandler := httpAPI.NewProductHandler(productUC, log)
	healthHandler := httpAPI.NewHealthHandler(healthUC, log)

	taskHandler.RegisterRoutes(router)
	productHandler.RegisterRoutes(router)
	healthHandler.RegisterRoutes(router)

	httpServer := &http.Server{
		Addr:    ":" + cfg.HTTP.Port,
		Handler: router,
	}

	return &App{
		cfg:            cfg,
		logger:         log,
		mongoClient:    mongoClient,
		rabbitClient:   rabbitClient,
		httpServer:     httpServer,
		parsingUseCase: parsingUC,
		productUseCase: productUC,
	}, nil
}

func (a *App) RunAPI(ctx context.Context) error {
	go func() {
		a.logger.Info("HTTP сервер запущен", "port", a.cfg.HTTP.Port)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("HTTP сервер упал", slog.Any("error", err))
		}
	}()

	a.logger.Info("API запущен. Ожидание сигналов...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	a.logger.Info("API останавливается...")

	// Создаем контекст для graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return a.Shutdown(shutdownCtx, "api")
}

func (a *App) RunWorkersOnly(ctx context.Context) error {
	a.logger.Info("Запуск воркеров...")
	go a.runMenuParsingConsumer(ctx)
	go a.runProductStatusConsumer(ctx)
	a.logger.Info("Воркеры запущены. Ожидание сигналов...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	a.logger.Info("Воркеры останавливаются...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return a.Shutdown(shutdownCtx, "worker")
}


func (a *App) Shutdown(shutdownCtx context.Context, mode string) error {
	var shutdownErr error 

	if mode == "api" {
		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			a.logger.Error("Ошибка остановки HTTP сервера", slog.Any("error", err))
			shutdownErr = errors.Join(shutdownErr, err)
		} else {
			a.logger.Info("HTTP сервер остановлен")
		}
	}

	if err := a.rabbitClient.Close(shutdownCtx); err != nil {
		a.logger.Error("Ошибка остановки RabbitMQ", slog.Any("error", err))
		shutdownErr = errors.Join(shutdownErr, err)
	} else {
		a.logger.Info("RabbitMQ остановлен")
	}

	if err := a.mongoClient.Disconnect(shutdownCtx); err != nil {
		a.logger.Error("Ошибка отключения от MongoDB", slog.Any("error", err))
		shutdownErr = errors.Join(shutdownErr, err)
	} else {
		a.logger.Info("MongoDB отключен")
	}

	return shutdownErr
}


func (a *App) runWorkers(ctx context.Context) {
    a.logger.Info("Запуск воркеров...")
    go a.runMenuParsingConsumer(ctx)
    go a.runProductStatusConsumer(ctx)
}

func (a *App) runMenuParsingConsumer(ctx context.Context) {
	queueName := "menu-parsing"

	// 1. Открываем изолированный канал
	ch, err := a.rabbitClient.OpenChannel()
	if err != nil {
		a.logger.Error(
			"Не удалось открыть канал для воркера",
			"queue", queueName,
			slog.Any("error", err),
		)
		return
	}
	defer ch.Close()

	// 2. Декларируем очередь и устанавливаем QoS
	if err := a.rabbitClient.SetupConsumerChannel(ctx, ch, queueName, true); err != nil {
		a.logger.Error(
			"Не удалось настроить канал потребителя",
			"queue", queueName,
			slog.Any("error", err),
		)
		return
	}

	// 3. Запускаем потребление (получаем канал доставки)
	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		a.logger.Error(
			"Не удалось зарегистрировать Consumer",
			"queue", queueName,
			slog.Any("error", err),
		)
		return
	}

	a.logger.Info("Consumer started successfully", "queue", queueName)

	// 4. Основной цикл обработки
	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-msgs:
			if !ok {
				a.logger.Warn(
					"RabbitMQ канал закрыт",
					"queue", queueName,
				)
				return
			}

			// Десериализация
			var message models.MenuParsingMessage
			if err := json.Unmarshal(msg.Body, &message); err != nil {
				a.logger.Error(
					"Poison pill: ошибка десериализации",
					slog.Any("error", err),
					slog.String("body", string(msg.Body)),
				)
				msg.Ack(false)
				continue
			}

			// Вызов UseCase
			if err := a.parsingUseCase.ProcessParsingTask(ctx, message); err != nil {
				// NACK → передаем в DLX
				a.logger.Error(
					"Processing failed, NACK sent",
					"task_id", message.TaskID,
					slog.Any("error", err),
				)
				msg.Nack(false, false) // не ре-очередить, передать DLX
			} else {
				// ACK → успех
				msg.Ack(false)
				a.logger.Info(
					"Task processed successfully",
					"task_id", message.TaskID,
				)
			}
		}
	}
}

func (a *App) runProductStatusConsumer(ctx context.Context) {
	queueName := "product-status"

	// 1. Открываем изолированный канал
	ch, err := a.rabbitClient.OpenChannel()
	if err != nil {
		a.logger.Error(
			"Failed to open channel",
			"queue", queueName,
			slog.Any("error", err),
		)
		return
	}
	defer ch.Close()

	// 2. Декларируем очередь и устанавливаем QoS
	if err := a.rabbitClient.SetupConsumerChannel(ctx, ch, queueName, true); err != nil {
		a.logger.Error(
			"Failed to setup consumer channel",
			"queue", queueName,
			slog.Any("error", err),
		)
		return
	}

	// 3. Запускаем потребление
	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		a.logger.Error(
			"Failed to register consumer",
			"queue", queueName,
			slog.Any("error", err),
		)
		return
	}

	a.logger.Info("Consumer started successfully", "queue", queueName)

	// 4. Основной цикл обработки
	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-msgs:
			if !ok {
				a.logger.Warn(
					"RabbitMQ channel closed",
					"queue", queueName,
				)
				return
			}

			// Десериализация
			var event models.ProductStatusEvent
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				a.logger.Error(
					"Poison pill: failed to unmarshal status event",
					slog.Any("error", err),
				)
				msg.Ack(false)
				continue
			}

			// Вызов UseCase
			if err := a.productUseCase.ProcessStatusUpdate(ctx, event); err != nil {
				// NACK → отправка в DLQ
				a.logger.Error(
					"Processing failed, NACK sent",
					"product_id", event.ProductID,
					slog.Any("error", err),
				)
				msg.Nack(false, false)
			} else {
				// ACK → успех
				msg.Ack(false)
				a.logger.Info(
					"Status event processed successfully",
					"product_id", event.ProductID,
				)
			}
		}
	}
}
