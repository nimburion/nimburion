//go:build !opensearch_sdk

package opensearch

import (
	"context"
	"fmt"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// OpenSearchSDKAdapter is available when built with the `opensearch_sdk` tag.
type OpenSearchSDKAdapter struct{}

// NewOpenSearchSDKAdapter returns an explanatory error when SDK support is not compiled in.
func NewOpenSearchSDKAdapter(cfg Config, log logger.Logger) (*OpenSearchSDKAdapter, error) {
	return nil, fmt.Errorf("opensearch-sdk adapter is not enabled; rebuild with `-tags opensearch_sdk`")
}

func (a *OpenSearchSDKAdapter) HealthCheck(ctx context.Context) error {
	return fmt.Errorf("opensearch-sdk adapter is not enabled; rebuild with `-tags opensearch_sdk`")
}

func (a *OpenSearchSDKAdapter) Close() error {
	return nil
}
