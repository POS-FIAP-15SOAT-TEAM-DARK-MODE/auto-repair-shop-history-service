// Package store is the DynamoDB read model of the History service (ddl.md, section `history`).
package store

import (
	"context"
	"errors"
	"time"
)

// ErrDuplicateMessage reports that a message_id was already applied; the caller discards the message.
var ErrDuplicateMessage = errors.New("message already processed")

// Status values of the Service Order and Work state machines that the store interprets.
const (
	WorkStatusInProgress = "IN_PROGRESS"
	WorkStatusCompleted  = "COMPLETED"
)

// StatusEvent is a ServiceOrderStatusChanged message, already mapped from the payload.
type StatusEvent struct {
	MessageID      string
	ServiceOrderID string
	PreviousStatus string // empty when the payload has previous = null
	NewStatus      string
	Reason         string // empty when null
	OccurredAt     time.Time
}

// WorkEvent is an ExecutionWorkStatusChanged message, already mapped from the payload.
type WorkEvent struct {
	MessageID      string
	ServiceOrderID string
	WorkID         string
	WorkName       string
	PreviousStatus string // empty when the payload has previous = null
	NewStatus      string
	OccurredAt     time.Time
}

// StatusRecord is a stored service-order transition.
type StatusRecord struct {
	PreviousStatus string
	NewStatus      string
	Reason         string
	CreatedAt      time.Time
}

// WorkRecord is a stored work transition of a service order.
type WorkRecord struct {
	WorkID         string
	WorkName       string
	PreviousStatus string
	NewStatus      string
	CreatedAt      time.Time
}

// ReportRecord is the aggregate of completed executions of one work.
type ReportRecord struct {
	WorkID       string
	WorkName     string
	Count        int64
	TotalSeconds float64
}

// Store is the read/write model used by the consumer (writes) and by the HTTP API (reads).
type Store interface {
	// WriteStatusEvent stores a service-order transition. Returns ErrDuplicateMessage when message_id was already applied.
	WriteStatusEvent(ctx context.Context, e StatusEvent) error
	// WriteWorkEvent stores a work transition and updates the report aggregate when applicable.
	// Returns ErrDuplicateMessage when message_id was already applied.
	WriteWorkEvent(ctx context.Context, e WorkEvent) error
	// ListByServiceOrder returns the status transitions and the work transitions of a service order, both in
	// chronological order.
	ListByServiceOrder(ctx context.Context, serviceOrderID string) ([]StatusRecord, []WorkRecord, error)
	// ListReport returns the report aggregates; an empty workIDs returns every work.
	ListReport(ctx context.Context, workIDs []string) ([]ReportRecord, error)
}
