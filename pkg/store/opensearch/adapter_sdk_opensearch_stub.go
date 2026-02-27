//go:build !opensearch_sdk

package opensearch

import (
	"context"
	"fmt"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// SDKAdapter is available when built with the `opensearch_sdk` tag.
type SDKAdapter struct{}

// NewSDKAdapter returns an explanatory error when SDK support is not compiled in.
func NewSDKAdapter(cfg Config, log logger.Logger) (*SDKAdapter, error) {
	return nil, fmt.Errorf("opensearch-sdk adapter is not enabled; rebuild with `-tags opensearch_sdk`")
}

func (a *SDKAdapter) HealthCheck(ctx context.Context) error {
	return fmt.Errorf("opensearch-sdk adapter is not enabled; rebuild with `-tags opensearch_sdk`")
}

func (a *SDKAdapter) Close() error {
	return nil
}
