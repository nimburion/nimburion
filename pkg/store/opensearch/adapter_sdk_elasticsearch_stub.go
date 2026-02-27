//go:build !elasticsearch_sdk

package opensearch

import (
	"context"
	"fmt"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// ElasticsearchSDKAdapter is available when built with the `elasticsearch_sdk` tag.
type ElasticsearchSDKAdapter struct{}

// NewElasticsearchSDKAdapter returns an explanatory error when SDK support is not compiled in.
func NewElasticsearchSDKAdapter(cfg Config, log logger.Logger) (*ElasticsearchSDKAdapter, error) {
	return nil, fmt.Errorf("elasticsearch-sdk adapter is not enabled; rebuild with `-tags elasticsearch_sdk`")
}

// HealthCheck verifies the component is operational and can perform its intended function.
func (a *ElasticsearchSDKAdapter) HealthCheck(ctx context.Context) error {
	return fmt.Errorf("elasticsearch-sdk adapter is not enabled; rebuild with `-tags elasticsearch_sdk`")
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (a *ElasticsearchSDKAdapter) Close() error {
	return nil
}
