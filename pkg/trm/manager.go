package trm

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// TxManager — это интерфейс нашего менеджера транзакций.
type TxManager interface {
	// Do выполняет функцию fn в контексте транзакции.
	Do(ctx context.Context, fn func(ctx context.Context) error) error
	
	// DoReadOnly выполняет функцию fn в контексте транза B c read-only.
	DoReadOnly(ctx context.Context, fn func(ctx context.Context) error) error
}

// Manager реализует TxManager для MongoDB.
type Manager struct {
	client *mongo.Client
}

// New возвращает новый Manager.
func New(client *mongo.Client) *Manager {
	return &Manager{client: client}
}

// Уникальный ключ для хранения опций в context
type ctxKeyTxOptions struct{}

var (
	txOptionsKey = ctxKeyTxOptions{}
)

// Do выполняет функцию fn в транзакции.
// Он автоматически обрабатывает Commit и Rollback.
func (m *Manager) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	
	// 1. Проверяем, не находимся ли мы *уже* внутри транзакции.
	// mongo.SessionFromContext извлечет сессию, если она была передана 
	// во 'fn' из родительского вызова 'Do'.
	if sess := mongo.SessionFromContext(ctx); sess != nil {
		// Мы уже в транзакции. MongoDB не поддерживает вложенные транзакции.
		// Просто выполняем функцию в *той же* транзакции.
		return fn(ctx)
	}

	// 2. Это вызов верхнего уровня. Начинаем новую сессию.
	session, err := m.client.StartSession()
	if err != nil {
		return fmt.Errorf("trm: не удалось запустить сессию: %w", err)
	}
	// Убедимся, что сессия закроется, когда 'Do' завершится
	defer session.EndSession(ctx) 

	// 3. Извлекаем опции транзакции из контекста (если есть)
	var txOpts *options.TransactionOptions
	if opts, ok := ctx.Value(txOptionsKey).(Options); ok {
		txOpts = toMongoOptions(opts)
	}

	// 4. Запускаем "Unit of Work" (нашу fn)
	// session.WithTransaction — это магическая функция.
	// Она сама запускает TX, выполняет наш код, 
	// и делает Commit (если нет ошибки) или Abort (если есть ошибка).
	_, err = session.WithTransaction(
		ctx,
		func(sessCtx mongo.SessionContext) (interface{}, error) {
			// sessCtx — это *новый* контекст, который несет в себе
			// информацию о транзакции.
			
			// Мы должны передать *именно его* в нашу функцию fn.
			if err := fn(sessCtx); err != nil {
				// Возврат ошибки здесь автоматически вызовет Rollback.
				return nil, err
			}
			
			// Возврат nil здесь автоматически вызовет Commit.
			return nil, nil
		},
		txOpts, // Передаем наши опции
	)

	// Возвращаем ошибку, если она была (Commit или Abort)
	return err
}

// DoReadOnly выполняет fn в read-only транзакции.
func (m *Manager) DoReadOnly(ctx context.Context, fn func(ctx context.Context) error) error {
	// Создаем опции для read-only
	roOpts := Options{
		ReadConcern:    readconcern.Majority(),
		ReadPreference: readpref.Primary(),
		WriteConcern:   writeconcern.New(writeconcern.WMajority()),
	}

	// Добавляем их в контекст
	ctx = WithOptions(ctx, roOpts)

	// Вызываем обычный 'Do', который их подхватит
	return m.Do(ctx, fn)
}

// Options — это наши абстрактные опции (DB-agnostic).
type Options struct {
	ReadConcern    *readconcern.ReadConcern
	WriteConcern   *writeconcern.WriteConcern
	ReadPreference *readpref.ReadPref
	MaxCommitTime  *time.Duration
}

// WithOptions сохраняет опции в контексте.
func WithOptions(ctx context.Context, opt Options) context.Context {
	return context.WithValue(ctx, txOptionsKey, opt)
}

// toMongoOptions конвертирует наши 'Options' в 'mongo.TransactionOptions'.
func toMongoOptions(o Options) *options.TransactionOptions {
	txOpts := options.Transaction()
	if o.ReadConcern != nil {
		txOpts.SetReadConcern(o.ReadConcern)
	}
	if o.WriteConcern != nil {
		txOpts.SetWriteConcern(o.WriteConcern)
	}
	if o.ReadPreference != nil {
		txOpts.SetReadPreference(o.ReadPreference)
	}
	if o.MaxCommitTime != nil {
		txOpts.SetMaxCommitTime(o.MaxCommitTime)
	}
	return txOpts
}