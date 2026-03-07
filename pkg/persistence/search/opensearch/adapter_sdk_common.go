package opensearch

import "fmt"

func collectAddresses(cfg Config) ([]string, error) {
	parsed, err := parseBaseURLs(cfg)
	if err != nil {
		return nil, err
	}

	addresses := make([]string, 0, len(parsed))
	for _, u := range parsed {
		addresses = append(addresses, u.String())
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("at least one search node address is required")
	}
	return addresses, nil
}
