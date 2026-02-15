package opensearch

import "testing"

func TestCollectAddresses_FromSingleAndMultipleURLs(t *testing.T) {
	addresses, err := collectAddresses(Config{
		URL:  "http://node-1:9200",
		URLs: []string{"http://node-2:9200", "http://node-1:9200"},
	})
	if err != nil {
		t.Fatalf("collect addresses: %v", err)
	}
	if len(addresses) != 2 {
		t.Fatalf("expected 2 unique addresses, got %d", len(addresses))
	}
}

func TestCollectAddresses_ValidationError(t *testing.T) {
	if _, err := collectAddresses(Config{}); err == nil {
		t.Fatal("expected error for missing urls")
	}
}
