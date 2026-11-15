//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/container"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/middleware"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/routing"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/consumer"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/history"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/auth"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	soA    = "01JD8Z4R3Q8M2V7K5N9XWAAAAA"
	soB    = "01JD8Z4R3Q8M2V7K5N9XWBBBBB"
	workX  = "11111111-1111-4111-8111-000000000001"
	workNm = "Troca de óleo"
)

var base = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

// fixture is one envelope to send through the queue.
type fixture struct {
	id, typ, so string
	at          time.Time
	payload     string
}

func (f fixture) body(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(messaging.Envelope{
		MessageID:      f.id,
		SagaID:         uuid.NewString(),
		Type:           f.typ,
		OccurredAt:     f.at,
		ServiceOrderID: f.so,
		RequestID:      uuid.NewString(),
		Payload:        json.RawMessage(f.payload),
	})
	require.NoError(t, err)
	return string(raw)
}

func soEvent(so, prev, next string, at time.Time) fixture {
	p := "null"
	if prev != "" {
		p = fmt.Sprintf("%q", prev)
	}
	return fixture{id: uuid.NewString(), typ: "ServiceOrderStatusChanged", so: so, at: at,
		payload: fmt.Sprintf(`{"previous":%s,"new":%q,"reason":null,"total_amount":"0.00"}`, p, next)}
}

func workEvent(so, prev, next string, at time.Time) fixture {
	p := "null"
	if prev != "" {
		p = fmt.Sprintf("%q", prev)
	}
	return fixture{id: uuid.NewString(), typ: "ExecutionWorkStatusChanged", so: so, at: at,
		payload: fmt.Sprintf(`{"work_id":%q,"work_name":%q,"previous":%s,"new":%q}`, workX, workNm, p, next)}
}

// sendAndDrain sends the messages to the queue, then receives and applies everything in it.
func sendAndDrain(t *testing.T, c *consumer.Consumer, q messaging.Queue, sqsClient *sqs.Client, queueURL string, msgs []fixture) {
	t.Helper()
	ctx := context.Background()
	for _, m := range msgs {
		_, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{QueueUrl: aws.String(queueURL), MessageBody: aws.String(m.body(t))})
		require.NoError(t, err)
	}
	empty := 0
	for empty < 3 {
		batch, err := q.ReceiveMessage(ctx, queueURL, 10, 1)
		require.NoError(t, err)
		if len(batch) == 0 {
			empty++
			continue
		}
		for _, m := range batch {
			require.NoError(t, c.Handle(ctx, m))
		}
	}
}

func TestHistoryAndReportEndToEnd(t *testing.T) {
	st, _, _ := newTestStore(t)
	ctx := context.Background()

	cfg, err := messaging.LoadAWSConfig(ctx)
	require.NoError(t, err)
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	sqsClient := messaging.NewSQSClient(cfg, endpoint)
	q := messaging.NewSQSQueue(sqsClient)

	out, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String("history-it-" + uuid.NewString())})
	require.NoError(t, err)
	queueURL := aws.ToString(out.QueueUrl)

	c := consumer.New(q, queueURL, st)

	// SO A: NEW -> IN_DIAGNOSIS -> IN_PROGRESS; work X runs from 12:02:30 to 14:32:30 (2.5 hours).
	// Transitions of the same work have distinct occurred_at: ties are ordered by message_id.
	soAEvents := []fixture{
		soEvent(soA, "", "NEW", base),
		soEvent(soA, "NEW", "IN_DIAGNOSIS", base.Add(1*time.Minute)),
		soEvent(soA, "IN_DIAGNOSIS", "IN_PROGRESS", base.Add(2*time.Minute)),
		workEvent(soA, "", "AWAITING_START", base.Add(2*time.Minute)),
		workEvent(soA, "AWAITING_START", "IN_PROGRESS", base.Add(2*time.Minute+30*time.Second)),
		workEvent(soA, "IN_PROGRESS", "COMPLETED", base.Add(152*time.Minute+30*time.Second)),
	}
	sendAndDrain(t, c, q, sqsClient, queueURL, soAEvents)

	// SO B: same work runs 1.5 hours; the duration is added to the same aggregate.
	soBEvents := []fixture{
		soEvent(soB, "", "NEW", base),
		soEvent(soB, "NEW", "IN_PROGRESS", base),
		workEvent(soB, "", "AWAITING_START", base),
		workEvent(soB, "AWAITING_START", "IN_PROGRESS", base),
		workEvent(soB, "IN_PROGRESS", "COMPLETED", base.Add(90*time.Minute)),
	}
	sendAndDrain(t, c, q, sqsClient, queueURL, soBEvents)

	// Redelivery of every message must not duplicate items nor add to the aggregate twice.
	sendAndDrain(t, c, q, sqsClient, queueURL, append(soAEvents, soBEvents...))

	svc := history.NewService(st)

	t.Run("history format matches the monolith", func(t *testing.T) {
		entries, err := svc.GetHistory(ctx, soA)
		require.NoError(t, err)
		require.Len(t, entries, 3)

		raw, err := json.Marshal(history.MapHistory(entries))
		require.NoError(t, err)
		assert.JSONEq(t, `[
		  {"previousStatus":"","newStatus":"NEW","createdAt":"2026-11-01T12:00:00Z"},
		  {"previousStatus":"NEW","newStatus":"IN_DIAGNOSIS","createdAt":"2026-11-01T12:01:00Z"},
		  {"previousStatus":"IN_DIAGNOSIS","newStatus":"IN_PROGRESS","createdAt":"2026-11-01T12:02:00Z",
		   "workTransitions":[{"workId":"`+workX+`","status":[
		     {"previousStatus":"","newStatus":"AWAITING_START","createdAt":"2026-11-01T12:02:00Z"},
		     {"previousStatus":"AWAITING_START","newStatus":"IN_PROGRESS","createdAt":"2026-11-01T12:02:30Z"},
		     {"previousStatus":"IN_PROGRESS","newStatus":"COMPLETED","createdAt":"2026-11-01T14:32:30Z"}
		   ]}]}
		]`, string(raw))
	})

	t.Run("unknown service order is empty", func(t *testing.T) {
		entries, err := svc.GetHistory(ctx, uuid.NewString())
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("report averages both executions once each", func(t *testing.T) {
		report, err := svc.GetAverageExecutionTime(ctx, []string{workX})
		require.NoError(t, err)
		require.Len(t, report, 1)
		assert.Equal(t, workNm, report[0].WorkName)
		// (2.5h + 1.5h) / 2 = 2h; a double count would give a different average.
		assert.InDelta(t, 2.0, report[0].AverageExecutionTimeHours, 1e-9)
	})

	t.Run("HTTP API returns the same contract", func(t *testing.T) {
		router := routing.SetupRouter(
			&container.HandlersWrapper{HistoryHandler: history.NewController(svc)},
			&container.Middlewares{"Recovery": middleware.Recovery()},
		)
		token, err := auth.GenerateToken("user-1", []auth.Role{auth.ATTENDANT}, time.Now().Add(time.Hour))
		require.NoError(t, err)

		get := func(path string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			return rec
		}

		rec := get("/v1/service-order/" + soA + "/history")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, strings.HasPrefix(rec.Body.String(), "[{"))

		rec = get("/v1/reports/average-execution-time?work_id=" + workX)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `[{"workId":"`+workX+`","workName":"`+workNm+`","averageExecutionTimeHours":2}]`, rec.Body.String())
	})
}
