package rabbit

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	Conn      *amqp.Connection
	Channel   *amqp.Channel
	closeChan chan *amqp.Error
	isClosed  bool
	mu        sync.Mutex
	dsn       string
}

// New creates RabbitMQ client
func New(ctx context.Context, dsn string) (*RabbitMQ, error) {
	conn, err := amqp.DialConfig(dsn, amqp.Config{
		Heartbeat: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create a channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// Create separate close channels
	connCloseChan := make(chan *amqp.Error, 1)
	chCloseChan := make(chan *amqp.Error, 1)
	conn.NotifyClose(connCloseChan)
	channel.NotifyClose(chCloseChan)

	// Merge both channels into one for monitoring
	mergedCloseChan := make(chan *amqp.Error, 2)
	go func() {
		for {
			select {
			case err := <-connCloseChan:
				if err != nil {
					log.Printf("[ERROR] RabbitMQ connection closed: %v", err)
					mergedCloseChan <- err
				} else {
					log.Println("[DEBUG] RabbitMQ connection closed gracefully")
					mergedCloseChan <- nil
				}
				return
			case err := <-chCloseChan:
				if err != nil {
					log.Printf("[ERROR] RabbitMQ channel closed: %v", err)
					mergedCloseChan <- err
				} else {
					log.Println("[DEBUG] RabbitMQ channel closed gracefully")
					mergedCloseChan <- nil
				}
				return
			}
		}
	}()

	log.Println("[INFO] Connected to RabbitMQ")

	r := &RabbitMQ{
		Conn:      conn,
		Channel:   channel,
		closeChan: mergedCloseChan,
		isClosed:  false,
		dsn:       dsn,
	}

	// Start monitoring connection in background
	go r.monitorConnection()

	return r, nil
}

// monitorConnection monitors the connection status
func (r *RabbitMQ) monitorConnection() {
	closeErr := <-r.closeChan
	r.isClosed = true

	if closeErr != nil {
		log.Printf("[ERROR] RabbitMQ connection closed with error: %v", closeErr)
	} else {
		log.Println("[DEBUG] RabbitMQ connection closed gracefully")
	}
}

// IsConnectionClosed checks if the connection is closed
func (r *RabbitMQ) IsConnectionClosed() bool {
	if r.Conn == nil {
		return true
	}
	return r.isClosed || r.Conn.IsClosed() || r.Channel.IsClosed()
}

// Close closes rabbit connection
func (r *RabbitMQ) Close(ctx context.Context) error {
	return r.closeWithContext(ctx)
}

// closeWithContext - closes RabbitMQ channel and connection using context
func (r *RabbitMQ) closeWithContext(ctx context.Context) error {
	log.Println("[DEBUG] Closing RabbitMQ channel")

	// Quick check under lock
	r.mu.Lock()
	if r.isClosed {
		r.mu.Unlock()
		return nil
	}

	// Mark closed early to avoid races with concurrent Close calls
	r.isClosed = true
	ch := r.Channel
	conn := r.Conn

	// Clear references so other goroutines know it's closed
	r.Channel = nil
	r.Conn = nil
	r.mu.Unlock()

	// Close channel first (if any)
	if ch != nil {
		if err := closeWithCtxFunc(ctx, ch.Close); err != nil {
			if ctx.Err() != nil {
				log.Println("[DEBUG] Context cancelled while closing channel")
			} else {
				log.Printf("[ERROR] Error closing channel: %v", err)
			}
		}
	}

	log.Println("[DEBUG] Closing RabbitMQ connection")

	// Close connection
	if conn != nil {
		if err := closeWithCtxFunc(ctx, conn.Close); err != nil {
			if ctx.Err() != nil {
				log.Println("[DEBUG] Context cancelled while closing connection")
				return ctx.Err()
			}
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	log.Println("[INFO] RabbitMQ closed")
	return nil
}

// helper to close a resource with context cancellation safely
func closeWithCtxFunc(ctx context.Context, fn func() error) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- fn()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Reconnect attempts to reconnect to RabbitMQ
func (r *RabbitMQ) Reconnect(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.dsn == "" {
		return fmt.Errorf("dsn is empty: can't reconnect")
	}

	if !r.isClosed && r.Conn != nil && !r.Conn.IsClosed() && r.Channel != nil && !r.Channel.IsClosed() {
		return nil
	}

	var conn *amqp.Connection
	var err error

	// Retry up to 5 times with exponential backoff
	for i := 0; i < 5; i++ {
		conn, err = amqp.DialConfig(r.dsn, amqp.Config{
			Heartbeat: 10 * time.Second,
		})
		if err == nil {
			break
		}

		wait := time.Duration(i+1) * 2 * time.Second
		log.Printf("[DEBUG] Reconnect attempt %d failed, retrying in %v", i+1, wait)

		select {
		case <-ctx.Done():
			log.Println("[DEBUG] Graceful shutdown — stopping reconnect attempts")
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	if err != nil {
		return fmt.Errorf("failed to reconnect to RabbitMQ after 5 attempts: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open a channel after reconnect: %w", err)
	}

	closeChan := make(chan *amqp.Error, 1)
	conn.NotifyClose(closeChan)

	r.Conn = conn
	r.Channel = ch
	r.closeChan = closeChan
	r.isClosed = false

	// Resume monitoring
	go r.monitorConnection()

	log.Println("[INFO] RabbitMQ reconnected successfully")
	return nil
}

// EnsureConnection checks connection and reconnects if needed
func (r *RabbitMQ) EnsureConnection(ctx context.Context) error {
	if r.IsConnectionClosed() {
		log.Println("[WARN] RabbitMQ connection closed, reconnecting...")
		if err := r.Reconnect(ctx); err != nil {
			return fmt.Errorf("failed to reconnect to RabbitMQ: %w", err)
		}
		log.Println("[INFO] RabbitMQ reconnected successfully")
	}
	return nil
}

// DeclareQueue declares a queue with Dead Letter Exchange support
func (r *RabbitMQ) DeclareQueue(ctx context.Context, queueName string, withDLX bool) error {
	if err := r.EnsureConnection(ctx); err != nil {
		return err
	}

	args := amqp.Table{}
	if withDLX {
		dlxName := queueName + "-dlx"
		dlqName := queueName + "-dlq"

		// Declare DLX
		if err := r.Channel.ExchangeDeclare(
			dlxName,
			"direct",
			true,  // durable
			false, // auto-deleted
			false, // internal
			false, // no-wait
			nil,
		); err != nil {
			return fmt.Errorf("failed to declare DLX: %w", err)
		}

		// Declare DLQ
		if _, err := r.Channel.QueueDeclare(
			dlqName,
			true,  // durable
			false, // delete when unused
			false, // exclusive
			false, // no-wait
			nil,
		); err != nil {
			return fmt.Errorf("failed to declare DLQ: %w", err)
		}

		// Bind DLQ to DLX
		if err := r.Channel.QueueBind(
			dlqName,
			queueName, // routing key
			dlxName,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind DLQ to DLX: %w", err)
		}

		args["x-dead-letter-exchange"] = dlxName
		args["x-dead-letter-routing-key"] = queueName
	}

	// Declare main queue
	_, err := r.Channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		args,
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}

	log.Printf("[INFO] Queue %s declared successfully", queueName)
	return nil
}

// Publish publishes a message to a queue
func (r *RabbitMQ) Publish(ctx context.Context, queueName string, body []byte) error {
	if err := r.EnsureConnection(ctx); err != nil {
		return err
	}

	return r.Channel.PublishWithContext(
		ctx,
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

// Consume starts consuming messages from a queue
func (r *RabbitMQ) Consume(ctx context.Context, queueName string, handler func([]byte) error) error {
	if err := r.EnsureConnection(ctx); err != nil {
		return err
	}

	// Set QoS to process one message at a time
	if err := r.Channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := r.Channel.Consume(
		queueName,
		"",    // consumer tag
		false, // auto-ack (manual ack required)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	log.Printf("[INFO] Started consuming from queue: %s", queueName)

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Context cancelled, stopping consumer")
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				log.Println("[WARN] Message channel closed, reconnecting...")
				if err := r.Reconnect(ctx); err != nil {
					return fmt.Errorf("failed to reconnect: %w", err)
				}
				// Restart consuming after reconnect
				return r.Consume(ctx, queueName, handler)
			}

			// Process message
			if err := handler(msg.Body); err != nil {
				log.Printf("[ERROR] Failed to process message: %v", err)
				// Requeue message with delay (using Nack)
				msg.Nack(false, true)
			} else {
				// Acknowledge message
				msg.Ack(false)
			}
		}
	}
}

// HealthCheck checks if RabbitMQ connection is healthy
func (r *RabbitMQ) HealthCheck(ctx context.Context) error {
	if r.IsConnectionClosed() {
		return fmt.Errorf("RabbitMQ connection is closed")
	}
	return nil
}

func (r *RabbitMQ) OpenChannel() (*amqp.Channel, error) {
	if r.Conn == nil {
		return nil, errors.New("connection is nil")
	}

	return r.Conn.Channel()
}

func (r *RabbitMQ) SetupConsumerChannel(ctx context.Context, ch *amqp.Channel, queueName string, withDLX bool) error {
	if err := ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	args := amqp.Table{}
	if withDLX {
		dlxName := queueName + "-dlx"
		dlqName := queueName + "-dlq"

		if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
			return fmt.Errorf("failed to declare DLX: %w", err)
		}
		if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
			return fmt.Errorf("failed to declare DLQ: %w", err)
		}

		if err := ch.QueueBind(dlqName, queueName, dlxName, false, nil); err != nil {
			return fmt.Errorf("failed to bind DLQ to DLX: %w", err)
		}

		args["x-dead-letter-exchange"] = dlxName
		args["x-dead-letter-routing-key"] = queueName
	}

	_, err := ch.QueueDeclare(queueName, true, false, false, false, args)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}

	return nil
}