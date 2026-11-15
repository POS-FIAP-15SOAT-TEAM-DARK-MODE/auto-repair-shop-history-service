// Package history builds the read API of the History service from the DynamoDB store.
package history

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
)

// ErrServiceOrderIDRequired is returned when the service order id is empty.
var ErrServiceOrderIDRequired = errors.New("service order id is required")

// StatusInProgress is the service-order status that carries the work transitions.
const StatusInProgress = "IN_PROGRESS"

// HistoryEntry is one service-order transition. WorkTransitions is set only on the IN_PROGRESS entry.
type HistoryEntry struct {
	PreviousStatus  string
	NewStatus       string
	CreatedAt       time.Time
	WorkTransitions []WorkTransitionGroup
}

// WorkTransitionGroup groups the transitions of one work in order of first appearance.
type WorkTransitionGroup struct {
	WorkID string
	Status []WorkStatusItem
}

// WorkStatusItem is one status change of a work.
type WorkStatusItem struct {
	PreviousStatus string
	NewStatus      string
	CreatedAt      time.Time
}

// ReportEntry is the average execution time of one work, in hours.
type ReportEntry struct {
	WorkID                    string
	WorkName                  string
	AverageExecutionTimeHours float64
}

// Service answers the History queries.
type Service struct {
	store store.Store
}

// NewService returns a Service backed by st.
func NewService(st store.Store) *Service {
	return &Service{store: st}
}

// GetHistory returns the transitions of a service order in chronological order. An unknown service order returns
// an empty slice.
func (s *Service) GetHistory(ctx context.Context, serviceOrderID string) ([]HistoryEntry, error) {
	if serviceOrderID == "" {
		return nil, ErrServiceOrderIDRequired
	}

	statuses, works, err := s.store.ListByServiceOrder(ctx, serviceOrderID)
	if err != nil {
		return nil, fmt.Errorf("fail to fetch service order history: %w", err)
	}

	entries := make([]HistoryEntry, 0, len(statuses))
	for _, st := range statuses {
		entries = append(entries, HistoryEntry{
			PreviousStatus: st.PreviousStatus,
			NewStatus:      st.NewStatus,
			CreatedAt:      st.CreatedAt,
		})
	}

	if groups := groupWorkTransitions(works); len(groups) > 0 {
		for i := range entries {
			if entries[i].NewStatus == StatusInProgress {
				entries[i].WorkTransitions = groups
				break
			}
		}
	}
	return entries, nil
}

// GetAverageExecutionTime returns the average IN_PROGRESS -> COMPLETED time per work, ordered by work name.
// An empty workIDs returns every work with at least one completed execution.
func (s *Service) GetAverageExecutionTime(ctx context.Context, workIDs []string) ([]ReportEntry, error) {
	records, err := s.store.ListReport(ctx, workIDs)
	if err != nil {
		return nil, fmt.Errorf("fail to fetch execution report: %w", err)
	}

	report := make([]ReportEntry, 0, len(records))
	for _, r := range records {
		if r.Count == 0 {
			continue
		}
		report = append(report, ReportEntry{
			WorkID:                    r.WorkID,
			WorkName:                  r.WorkName,
			AverageExecutionTimeHours: r.TotalSeconds / float64(r.Count) / 3600,
		})
	}
	sort.SliceStable(report, func(i, j int) bool {
		if report[i].WorkName != report[j].WorkName {
			return report[i].WorkName < report[j].WorkName
		}
		return report[i].WorkID < report[j].WorkID
	})
	return report, nil
}

// groupWorkTransitions orders the work transitions chronologically and groups them by work id. Groups are ordered by
// the first appearance of each work; statuses inside a group keep chronological order.
func groupWorkTransitions(works []store.WorkRecord) []WorkTransitionGroup {
	sorted := make([]store.WorkRecord, len(works))
	copy(sorted, works)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	var groups []WorkTransitionGroup
	index := make(map[string]int)
	for _, w := range sorted {
		i, ok := index[w.WorkID]
		if !ok {
			i = len(groups)
			index[w.WorkID] = i
			groups = append(groups, WorkTransitionGroup{WorkID: w.WorkID})
		}
		groups[i].Status = append(groups[i].Status, WorkStatusItem{
			PreviousStatus: w.PreviousStatus,
			NewStatus:      w.NewStatus,
			CreatedAt:      w.CreatedAt,
		})
	}
	return groups
}
