package document

import (
	"testing"

	dynamostore "github.com/nimburion/nimburion/pkg/store/dynamodb"
	mongostore "github.com/nimburion/nimburion/pkg/store/mongodb"
)

func TestSortOrder_Constants(t *testing.T) {
	if SortAsc != "asc" {
		t.Fatalf("SortAsc = %q, want asc", SortAsc)
	}
	if SortDesc != "desc" {
		t.Fatalf("SortDesc = %q, want desc", SortDesc)
	}
}

func TestNewDynamoDBExecutor_Validation(t *testing.T) {
	if _, err := NewDynamoDBExecutor(nil); err == nil {
		t.Fatal("expected error for nil adapter")
	}

	exec, err := NewDynamoDBExecutor(&dynamostore.DynamoDBAdapter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestNewMongoDBExecutor_Validation(t *testing.T) {
	if _, err := NewMongoDBExecutor(nil); err == nil {
		t.Fatal("expected error for nil adapter")
	}

	exec, err := NewMongoDBExecutor(&mongostore.MongoDBAdapter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec == nil {
		t.Fatal("expected non-nil executor")
	}
}
