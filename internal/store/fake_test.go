package store

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// fakeAPI records the transactions and answers queries and reads with canned responses.
type fakeAPI struct {
	transactions []*dynamodb.TransactWriteItemsInput
	transactErr  error

	getOut *dynamodb.GetItemOutput
	getErr error

	queryFn    func(in *dynamodb.QueryInput) (*dynamodb.QueryOutput, error)
	queryCalls []*dynamodb.QueryInput
}

func (f *fakeAPI) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	f.transactions = append(f.transactions, in)
	if f.transactErr != nil {
		return nil, f.transactErr
	}
	return &dynamodb.TransactWriteItemsOutput{}, nil
}

func (f *fakeAPI) GetItem(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.getOut == nil {
		return &dynamodb.GetItemOutput{}, nil
	}
	return f.getOut, nil
}

func (f *fakeAPI) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	f.queryCalls = append(f.queryCalls, in)
	if f.queryFn == nil {
		return &dynamodb.QueryOutput{}, nil
	}
	return f.queryFn(in)
}

// cancelled builds the error DynamoDB returns when the transaction is cancelled, one reason per item.
func cancelled(codes ...string) error {
	reasons := make([]types.CancellationReason, 0, len(codes))
	for _, c := range codes {
		reasons = append(reasons, types.CancellationReason{Code: strPtr(c)})
	}
	return &types.TransactionCanceledException{CancellationReasons: reasons}
}

var errBoom = errors.New("boom")

func strPtr(s string) *string { return &s }

// s and n build attribute values for canned items.
func s(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }
func n(v string) types.AttributeValue { return &types.AttributeValueMemberN{Value: v} }
