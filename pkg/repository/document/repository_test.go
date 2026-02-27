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

	exec, err := NewDynamoDBExecutor(&dynamostore.Adapter{})
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

	exec, err := NewMongoDBExecutor(&mongostore.Adapter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestFilter(t *testing.T) {
	f := Filter{"status": "active", "age": 25}
	if len(f) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(f))
	}
	if f["status"] != "active" {
		t.Fatalf("expected status=active, got %v", f["status"])
	}
}

func TestSort(t *testing.T) {
	s := Sort{Field: "created_at", Order: SortDesc}
	if s.Field != "created_at" {
		t.Fatalf("expected field=created_at, got %s", s.Field)
	}
	if s.Order != SortDesc {
		t.Fatalf("expected order=desc, got %s", s.Order)
	}
}

func TestPagination(t *testing.T) {
	p := Pagination{Limit: 10, Cursor: "abc123"}
	if p.Limit != 10 {
		t.Fatalf("expected limit=10, got %d", p.Limit)
	}
	if p.Cursor != "abc123" {
		t.Fatalf("expected cursor=abc123, got %s", p.Cursor)
	}
}

func TestQueryOptions(t *testing.T) {
	opts := QueryOptions{
		Filter:     Filter{"status": "active"},
		Sort:       Sort{Field: "name", Order: SortAsc},
		Pagination: Pagination{Limit: 20},
	}
	if opts.Filter["status"] != "active" {
		t.Fatal("filter not set correctly")
	}
	if opts.Sort.Field != "name" {
		t.Fatal("sort not set correctly")
	}
	if opts.Pagination.Limit != 20 {
		t.Fatal("pagination not set correctly")
	}
}
