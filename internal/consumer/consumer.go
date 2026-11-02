// Package consumer reads the History queue and applies ServiceOrderStatusChanged and ExecutionWorkStatusChanged
// messages to the store. Every other message type is discarded.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/logger"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/messaging"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"go.uber.org/zap"
)

// Message types handled by the History service (messaging.md, "Quem trata o quê").
const (
	TypeServiceOrderStatusChanged  = "ServiceOrderStatusChanged"
	TypeExecutionWorkStatusChanged = "ExecutionWorkStatusChanged"
)

// ErrInvalidPayload is returned for a known type whose payload cannot be applied. The message stays in the queue
// and reaches the DLQ after maxReceiveCount attempts.
var ErrInvalidPayload = errors.New("invalid message payload")

// Consumer receives messages from SQS and writes them to the store.
//
// A message is deleted only after it was applied, found to be a duplicate, or discarded because its type is not
// handled. Any other error leaves it in the queue.
type Consumer struct {
	queue    messaging.Queue
	queueURL string
	store    store.Store
}

// New returns a consumer for queueURL.
func New(queue messaging.Queue, queueURL string, st store.Store) *Consumer {
	return &Consumer{queue: queue, queueURL: queueURL, store: st}
}

// Run long-polls the queue until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		msgs, err := c.queue.ReceiveMessage(ctx, c.queueURL, 10, 20)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			logger.Of(ctx).ErrorEvent("queue.receive_failed", err, zap.String("component", "sqs-consumer"))
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
			continue
		}
		for _, m := range msgs {
			// Handle logs its own failures, with the message fields in the context.
			_ = c.Handle(ctx, m)
		}
	}
	return nil
}

// Handle processes one SQS message. A non-nil error means the message must stay in the queue. Every failure is logged
// here, once, with the message_id, saga_id and service_order_id of the envelope in the logger context.
func (c *Consumer) Handle(ctx context.Context, m messaging.Message) error {
	env, err := messaging.Decode([]byte(m.Body))
	if err != nil {
		err = fmt.Errorf("decode message: %w", err)
		logger.Of(ctx).ErrorEvent("message.invalid", err, zap.String("component", "sqs-consumer"))
		return err
	}
	ctx = logger.WithFields(ctx,
		zap.String("message_id", env.MessageID),
		zap.String("saga_id", env.SagaID),
		zap.String("service_order_id", env.ServiceOrderID),
	)

	if err := c.apply(ctx, env); err != nil {
		if !errors.Is(err, store.ErrDuplicateMessage) {
			err = fmt.Errorf("apply %s: %w", env.Type, err)
			logger.Of(ctx).ErrorEvent("message.failed", err,
				zap.String("type", env.Type), zap.String("component", "sqs-consumer"))
			return err
		}
		logger.Of(ctx).Info("message.duplicate", zap.String("type", env.Type))
	} else {
		logger.Of(ctx).Info("message.applied", zap.String("type", env.Type))
	}

	if err := c.queue.DeleteMessage(ctx, c.queueURL, m.ReceiptHandle); err != nil {
		err = fmt.Errorf("delete message: %w", err)
		logger.Of(ctx).ErrorEvent("message.delete_failed", err, zap.String("component", "sqs-consumer"))
		return err
	}
	return nil
}

// apply routes by type. Unknown types are logged and treated as applied so they are removed from the queue.
func (c *Consumer) apply(ctx context.Context, env messaging.Envelope) error {
	switch env.Type {
	case TypeServiceOrderStatusChanged:
		e, err := serviceOrderEvent(env)
		if err != nil {
			return err
		}
		return c.store.WriteStatusEvent(ctx, e)
	case TypeExecutionWorkStatusChanged:
		e, err := workEvent(env)
		if err != nil {
			return err
		}
		return c.store.WriteWorkEvent(ctx, e)
	default:
		logger.Of(ctx).Warn("message.discarded",
			zap.String("type", env.Type), zap.String("reason", "unhandled_type"))
		return nil
	}
}

type serviceOrderPayload struct {
	Previous *string `json:"previous"`
	New      string  `json:"new"`
	Reason   *string `json:"reason"`
}

type workPayload struct {
	WorkID   string  `json:"work_id"`
	WorkName string  `json:"work_name"`
	Previous *string `json:"previous"`
	New      string  `json:"new"`
}

func serviceOrderEvent(env messaging.Envelope) (store.StatusEvent, error) {
	var p serviceOrderPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return store.StatusEvent{}, errors.Join(ErrInvalidPayload, err)
	}
	if p.New == "" {
		return store.StatusEvent{}, fmt.Errorf("%w: new is required", ErrInvalidPayload)
	}
	return store.StatusEvent{
		MessageID:      env.MessageID,
		ServiceOrderID: env.ServiceOrderID,
		PreviousStatus: deref(p.Previous),
		NewStatus:      p.New,
		Reason:         deref(p.Reason),
		OccurredAt:     env.OccurredAt,
	}, nil
}

func workEvent(env messaging.Envelope) (store.WorkEvent, error) {
	var p workPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return store.WorkEvent{}, errors.Join(ErrInvalidPayload, err)
	}
	if p.WorkID == "" || p.WorkName == "" || p.New == "" {
		return store.WorkEvent{}, fmt.Errorf("%w: work_id, work_name and new are required", ErrInvalidPayload)
	}
	return store.WorkEvent{
		MessageID:      env.MessageID,
		ServiceOrderID: env.ServiceOrderID,
		WorkID:         p.WorkID,
		WorkName:       p.WorkName,
		PreviousStatus: deref(p.Previous),
		NewStatus:      p.New,
		OccurredAt:     env.OccurredAt,
	}, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
