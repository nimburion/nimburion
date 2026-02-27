package openapivalidation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	logpkg "github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/server/router"
)

const (
	ValidationModeStrict   = "strict"
	ValidationModeWarnOnly = "warn-only"
)

// Config configures OpenAPI request validation.
type Config struct {
	SpecPath    string
	StripPrefix string
	Mode        string // strict, warn-only
}

// NewRequestValidationMiddleware validates incoming requests against an OpenAPI document.
func NewRequestValidationMiddleware(cfg Config, log logpkg.Logger) (router.MiddlewareFunc, error) {
	specPath := strings.TrimSpace(cfg.SpecPath)
	if specPath == "" {
		return nil, fmt.Errorf("openapi spec path is empty")
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("load openapi spec %s: %w", specPath, err)
	}
	if validateErr := doc.Validate(context.Background()); validateErr != nil {
		return nil, fmt.Errorf("validate openapi spec %s: %w", specPath, validateErr)
	}

	specRouter, err := legacyrouter.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build openapi router %s: %w", specPath, err)
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = ValidationModeStrict
	}

	options := &openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			req := c.Request()

			bodyBytes, bodyReadErr := snapshotAndRestoreRequestBody(req)
			if bodyReadErr != nil {
				validationErr := fmt.Errorf("read request body for validation: %w", bodyReadErr)
				if mode != ValidationModeWarnOnly {
					return c.JSON(openapiValidationStatusCode(validationErr), map[string]string{
						"error":  "request validation failed",
						"detail": validationErr.Error(),
					})
				}
				warnValidationFailure(log, specPath, req, validationErr)
				return next(c)
			}

			validationReq := cloneRequestForValidation(req, bodyBytes, cfg.StripPrefix)
			if err := validateOpenAPIRequest(validationReq, specRouter, options); err != nil {
				if mode != ValidationModeWarnOnly {
					return c.JSON(openapiValidationStatusCode(err), map[string]string{
						"error":  "request validation failed",
						"detail": err.Error(),
					})
				}
				warnValidationFailure(log, specPath, req, err)
			}

			return next(c)
		}
	}, nil
}

func warnValidationFailure(log logpkg.Logger, specPath string, req *http.Request, err error) {
	if log == nil || req == nil || err == nil {
		return
	}

	log.Warn(
		"openapi request validation failed (warn-only)",
		"spec_path", specPath,
		"method", req.Method,
		"path", req.URL.Path,
		"error", err.Error(),
	)
}

func snapshotAndRestoreRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}

	body, err := io.ReadAll(req.Body)
	if closeErr := req.Body.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))

	return body, nil
}

func cloneRequestForValidation(req *http.Request, body []byte, stripPrefix string) *http.Request {
	validationReq := req.Clone(req.Context())
	if req.URL != nil {
		urlCopy := *req.URL
		validationReq.URL = &urlCopy
	}

	trimmedStripPrefix := strings.TrimSpace(stripPrefix)
	if trimmedStripPrefix != "" && validationReq.URL != nil && strings.HasPrefix(validationReq.URL.Path, trimmedStripPrefix) {
		rewritten := strings.TrimPrefix(validationReq.URL.Path, trimmedStripPrefix)
		if rewritten == "" {
			rewritten = "/"
		}
		validationReq.URL.Path = rewritten
		if validationReq.URL.RawPath != "" {
			validationReq.URL.RawPath = rewritten
		}
	}

	if len(body) == 0 {
		validationReq.Body = http.NoBody
		validationReq.ContentLength = 0
		return validationReq
	}

	validationReq.Body = io.NopCloser(bytes.NewReader(body))
	validationReq.ContentLength = int64(len(body))
	return validationReq
}

func validateOpenAPIRequest(req *http.Request, specRouter routers.Router, opts *openapi3filter.Options) error {
	route, pathParams, err := specRouter.FindRoute(req)
	if err != nil {
		return err
	}

	input := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options:    opts,
	}
	return openapi3filter.ValidateRequest(req.Context(), input)
}

func openapiValidationStatusCode(err error) int {
	if errors.Is(err, routers.ErrMethodNotAllowed) {
		return http.StatusMethodNotAllowed
	}
	return http.StatusBadRequest
}
