package repository

import "testing"

func TestPagination_Offset(t *testing.T) {
	tests := []struct {
		name string
		p    Pagination
		want int
	}{
		{name: "first page", p: Pagination{Page: 1, PageSize: 10}, want: 0},
		{name: "second page", p: Pagination{Page: 2, PageSize: 10}, want: 10},
		{name: "invalid page", p: Pagination{Page: 0, PageSize: 10}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Offset(); got != tt.want {
				t.Fatalf("offset = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPagination_Limit(t *testing.T) {
	p := Pagination{Page: 3, PageSize: 25}
	if got := p.Limit(); got != 25 {
		t.Fatalf("limit = %d, want 25", got)
	}
}

func TestSortOrder_Constants(t *testing.T) {
	if SortAsc != "asc" {
		t.Fatalf("SortAsc = %q, want asc", SortAsc)
	}
	if SortDesc != "desc" {
		t.Fatalf("SortDesc = %q, want desc", SortDesc)
	}
}
