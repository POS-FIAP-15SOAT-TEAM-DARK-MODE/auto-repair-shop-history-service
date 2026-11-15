//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"github.com/google/uuid"
)

// TestTransactWriteItemsOnMiniStack checks that the dedupe condition of TransactWriteItems is enforced by the
// emulator: the second write of the same message_id is rejected as a duplicate and writes nothing.
func TestTransactWriteItemsOnMiniStack(t *testing.T) {
	st, _, _ := newTestStore(t)
	ctx := context.Background()

	event := store.StatusEvent{
		MessageID:      uuid.NewString(),
		ServiceOrderID: uuid.NewString(),
		NewStatus:      "NEW",
		OccurredAt:     time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
	}

	if err := st.WriteStatusEvent(ctx, event); err != nil {
		t.Fatalf("first write: %v", err)
	}
	err := st.WriteStatusEvent(ctx, event)
	if !errors.Is(err, store.ErrDuplicateMessage) {
		t.Fatalf("second write: want ErrDuplicateMessage, got %v", err)
	}

	statuses, _, err := st.ListByServiceOrder(ctx, event.ServiceOrderID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("want 1 status item after duplicate, got %d", len(statuses))
	}
}
