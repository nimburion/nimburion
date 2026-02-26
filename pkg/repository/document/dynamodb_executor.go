package document

import (
	"context"
	"fmt"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	dynamostore "github.com/nimburion/nimburion/pkg/store/dynamodb"
)

// DynamoExecutor defines a minimal document execution contract for DynamoDB-backed repositories.
// Expressions are intentionally passed-through to keep full DynamoDB capabilities available.
type DynamoExecutor interface {
	PutItem(ctx context.Context, table string, item map[string]types.AttributeValue) error
	GetItem(ctx context.Context, table string, key map[string]types.AttributeValue, consistentRead bool) (map[string]types.AttributeValue, error)
	UpdateItem(
		ctx context.Context,
		table string,
		key map[string]types.AttributeValue,
		updateExpression string,
		expressionNames map[string]string,
		expressionValues map[string]types.AttributeValue,
	) (map[string]types.AttributeValue, error)
	DeleteItem(ctx context.Context, table string, key map[string]types.AttributeValue) error
	Query(
		ctx context.Context,
		table string,
		keyConditionExpression string,
		expressionNames map[string]string,
		expressionValues map[string]types.AttributeValue,
		limit int32,
		exclusiveStartKey map[string]types.AttributeValue,
	) ([]map[string]types.AttributeValue, map[string]types.AttributeValue, error)
}

// DynamoDBExecutor adapts store/dynamodb adapter to the repository/document executor contract.
type DynamoDBExecutor struct {
	adapter *dynamostore.DynamoDBAdapter
}

// NewDynamoDBExecutor creates a new DynamoDBExecutor instance.
func NewDynamoDBExecutor(adapter *dynamostore.DynamoDBAdapter) (*DynamoDBExecutor, error) {
	if adapter == nil {
		return nil, fmt.Errorf("dynamodb adapter is required")
	}
	return &DynamoDBExecutor{adapter: adapter}, nil
}

// PutItem TODO: add description
func (e *DynamoDBExecutor) PutItem(ctx context.Context, table string, item map[string]types.AttributeValue) error {
	_, err := e.adapter.PutItem(ctx, &awsdynamodb.PutItemInput{
		TableName: &table,
		Item:      item,
	})
	return err
}

// GetItem TODO: add description
func (e *DynamoDBExecutor) GetItem(ctx context.Context, table string, key map[string]types.AttributeValue, consistentRead bool) (map[string]types.AttributeValue, error) {
	out, err := e.adapter.GetItem(ctx, &awsdynamodb.GetItemInput{
		TableName:      &table,
		Key:            key,
		ConsistentRead: &consistentRead,
	})
	if err != nil {
		return nil, err
	}
	return out.Item, nil
}

// UpdateItem TODO: add description
func (e *DynamoDBExecutor) UpdateItem(
	ctx context.Context,
	table string,
	key map[string]types.AttributeValue,
	updateExpression string,
	expressionNames map[string]string,
	expressionValues map[string]types.AttributeValue,
) (map[string]types.AttributeValue, error) {
	out, err := e.adapter.UpdateItem(ctx, &awsdynamodb.UpdateItemInput{
		TableName:                 &table,
		Key:                       key,
		UpdateExpression:          &updateExpression,
		ExpressionAttributeNames:  expressionNames,
		ExpressionAttributeValues: expressionValues,
		ReturnValues:              types.ReturnValueAllNew,
	})
	if err != nil {
		return nil, err
	}
	return out.Attributes, nil
}

// DeleteItem TODO: add description
func (e *DynamoDBExecutor) DeleteItem(ctx context.Context, table string, key map[string]types.AttributeValue) error {
	_, err := e.adapter.DeleteItem(ctx, &awsdynamodb.DeleteItemInput{
		TableName: &table,
		Key:       key,
	})
	return err
}

// Query retrieves a URL query parameter by name.
func (e *DynamoDBExecutor) Query(
	ctx context.Context,
	table string,
	keyConditionExpression string,
	expressionNames map[string]string,
	expressionValues map[string]types.AttributeValue,
	limit int32,
	exclusiveStartKey map[string]types.AttributeValue,
) ([]map[string]types.AttributeValue, map[string]types.AttributeValue, error) {
	out, err := e.adapter.Query(ctx, &awsdynamodb.QueryInput{
		TableName:                 &table,
		KeyConditionExpression:    &keyConditionExpression,
		ExpressionAttributeNames:  expressionNames,
		ExpressionAttributeValues: expressionValues,
		Limit:                     &limit,
		ExclusiveStartKey:         exclusiveStartKey,
	})
	if err != nil {
		return nil, nil, err
	}
	return out.Items, out.LastEvaluatedKey, nil
}
