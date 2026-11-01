package bootstrap

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/container"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/middleware"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/app/routing"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/consumer"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/history"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/env"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/logger"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/messaging"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"go.uber.org/zap"
)

// Servers returns the HTTP API and, when HISTORY_QUEUE_URL is set, the SQS consumer.
func Servers() []app.Server {
	ctx := context.Background()

	table := env.GetString("DYNAMODB_TABLE", "")
	if table == "" {
		logger.Global().Fatal("invalid configuration", zap.String("missing", "DYNAMODB_TABLE"))
	}

	awsCfg, err := messaging.LoadAWSConfig(ctx)
	if err != nil {
		logger.Global().Fatal("failed to load AWS config", zap.Error(err))
	}
	endpoint := env.GetString("AWS_ENDPOINT_URL", "")
	st := store.NewDynamoStore(store.NewDynamoDBClient(awsCfg, endpoint), table)

	router := routing.SetupRouter(
		&container.HandlersWrapper{HistoryHandler: history.NewController(history.NewService(st))},
		middlewaresContainer(),
	)

	servers := []app.Server{app.NewHTTPServer("http-api", ":"+env.GetString("PORT", "8085"), router)}
	return append(servers, messagingWorkers(awsCfg, endpoint, st)...)
}

// messagingWorkers starts the consumer, disabled when HISTORY_QUEUE_URL is empty.
func messagingWorkers(awsCfg aws.Config, endpoint string, st store.Store) []app.Server {
	queueURL := env.GetString("HISTORY_QUEUE_URL", "")
	if queueURL == "" {
		logger.Global().Warn("messaging disabled: HISTORY_QUEUE_URL is empty")
		return nil
	}

	queue := messaging.NewSQSQueue(messaging.NewSQSClient(awsCfg, endpoint))
	c := consumer.New(queue, queueURL, st)
	return []app.Server{newWorker("sqs-consumer", c.Run)}
}

func middlewaresContainer() *container.Middlewares {
	return &container.Middlewares{
		"Logger":   middleware.Logger(),
		"Recovery": middleware.Recovery(),
		"Metrics":  middleware.Metrics(),
	}
}
