package history

import "time"

// The JSON shapes below are the same as the monolith's (service_order_history/adapters/so_history_read.go).

// WorkStatusEntryDTO is a work status change in the history response.
type WorkStatusEntryDTO struct {
	PreviousStatus string    `json:"previousStatus"`
	NewStatus      string    `json:"newStatus"`
	CreatedAt      time.Time `json:"createdAt"`
}

// WorkTransitionDTO is one work and its status changes.
type WorkTransitionDTO struct {
	WorkID string               `json:"workId"`
	Status []WorkStatusEntryDTO `json:"status"`
}

// HistoryDTO is one service-order transition in the history response.
type HistoryDTO struct {
	PreviousStatus  string              `json:"previousStatus"`
	NewStatus       string              `json:"newStatus"`
	CreatedAt       time.Time           `json:"createdAt"`
	WorkTransitions []WorkTransitionDTO `json:"workTransitions,omitempty"`
}

// ReportDTO is one work in the average execution time response.
type ReportDTO struct {
	WorkID                    string  `json:"workId"`
	WorkName                  string  `json:"workName"`
	AverageExecutionTimeHours float64 `json:"averageExecutionTimeHours"`
}

// MapHistory converts entries to DTOs. It never returns nil, so an empty history is serialized as [].
func MapHistory(entries []HistoryEntry) []HistoryDTO {
	out := make([]HistoryDTO, 0, len(entries))
	for _, e := range entries {
		dto := HistoryDTO{PreviousStatus: e.PreviousStatus, NewStatus: e.NewStatus, CreatedAt: e.CreatedAt}
		for _, g := range e.WorkTransitions {
			statuses := make([]WorkStatusEntryDTO, 0, len(g.Status))
			for _, s := range g.Status {
				statuses = append(statuses, WorkStatusEntryDTO{
					PreviousStatus: s.PreviousStatus,
					NewStatus:      s.NewStatus,
					CreatedAt:      s.CreatedAt,
				})
			}
			dto.WorkTransitions = append(dto.WorkTransitions, WorkTransitionDTO{WorkID: g.WorkID, Status: statuses})
		}
		out = append(out, dto)
	}
	return out
}

// MapReport converts report entries to DTOs. It never returns nil, so an empty report is serialized as [].
func MapReport(entries []ReportEntry) []ReportDTO {
	out := make([]ReportDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, ReportDTO{
			WorkID:                    e.WorkID,
			WorkName:                  e.WorkName,
			AverageExecutionTimeHours: e.AverageExecutionTimeHours,
		})
	}
	return out
}
