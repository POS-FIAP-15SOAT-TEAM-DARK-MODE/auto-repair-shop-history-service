#!/bin/sh
# Idempotent local setup of the History service resources on MiniStack.
# Creates the three event topics (they are owned by other services; History only subscribes), the history queue with
# its DLQ, the queue policy, the subscriptions with raw delivery, and the DynamoDB table.
set -eu

P=auto-repair-shop-local
ENDPOINT="${AWS_ENDPOINT_URL:-http://ministack:4566}"
REGION="${AWS_REGION:-us-east-1}"
AWS="aws --endpoint-url $ENDPOINT --region $REGION"

# Topics the History service listens to (created here so the subscriptions below work on a fresh emulator)
SO_ARN=$($AWS sns create-topic --name "$P-so-events" --query TopicArn --output text)
BILLING_ARN=$($AWS sns create-topic --name "$P-billing-events" --query TopicArn --output text)
EXECUTION_ARN=$($AWS sns create-topic --name "$P-execution-events" --query TopicArn --output text)

# Queue with DLQ (maxReceiveCount=5)
DLQ_URL=$($AWS sqs create-queue --queue-name "$P-history-queue-dlq" --query QueueUrl --output text)
DLQ_ARN=$($AWS sqs get-queue-attributes --queue-url "$DLQ_URL" --attribute-names QueueArn --query Attributes.QueueArn --output text)
QUEUE_URL=$($AWS sqs create-queue --queue-name "$P-history-queue" --query QueueUrl --output text)
QUEUE_ARN=$($AWS sqs get-queue-attributes --queue-url "$QUEUE_URL" --attribute-names QueueArn --query Attributes.QueueArn --output text)

REDRIVE="{\"deadLetterTargetArn\":\"$DLQ_ARN\",\"maxReceiveCount\":\"5\"}"
$AWS sqs set-queue-attributes --queue-url "$QUEUE_URL" --attributes "RedrivePolicy=$REDRIVE" || true

# Queue policy: only the three event topics may send to the queue
POLICY="{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Principal\":\"*\",\"Action\":\"sqs:SendMessage\",\"Resource\":\"$QUEUE_ARN\",\"Condition\":{\"ArnEquals\":{\"aws:SourceArn\":[\"$SO_ARN\",\"$BILLING_ARN\",\"$EXECUTION_ARN\"]}}}]}"
$AWS sqs set-queue-attributes --queue-url "$QUEUE_URL" --attributes "Policy=$POLICY" || true

# Subscriptions with raw message delivery. No filter policy: the consumer discards the types it does not use.
for TOPIC_ARN in "$SO_ARN" "$BILLING_ARN" "$EXECUTION_ARN"; do
  SUB_ARN=$($AWS sns subscribe --topic-arn "$TOPIC_ARN" --protocol sqs --notification-endpoint "$QUEUE_ARN" --query SubscriptionArn --output text)
  $AWS sns set-subscription-attributes --subscription-arn "$SUB_ARN" --attribute-name RawMessageDelivery --attribute-value true || true
done

# DynamoDB read model (keys PK/SK, on-demand)
$AWS dynamodb create-table \
  --table-name "$P-history" \
  --attribute-definitions AttributeName=PK,AttributeType=S AttributeName=SK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH AttributeName=SK,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST >/dev/null 2>&1 || true

echo "history ready: queue=$QUEUE_URL table=$P-history"
