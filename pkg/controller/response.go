package controller

import (
	"net/http"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// SuccessResponse represents a successful response with data
type SuccessResponse struct {
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id,omitempty"`
}

// Success sends a successful JSON response with HTTP 200 OK
// It wraps the provided data in a consistent response format
func Success(c router.Context, data interface{}) error {
	requestID := getRequestID(c.Request().Context())
	return c.JSON(http.StatusOK, SuccessResponse{
		Data:      data,
		RequestID: requestID,
	})
}

// Created sends a successful JSON response with HTTP 201 Created
// It wraps the provided data in a consistent response format
// Typically used after successfully creating a new resource
func Created(c router.Context, data interface{}) error {
	requestID := getRequestID(c.Request().Context())
	return c.JSON(http.StatusCreated, SuccessResponse{
		Data:      data,
		RequestID: requestID,
	})
}

// NoContent sends a successful response with HTTP 204 No Content
// No response body is sent
// Typically used after successfully deleting a resource or updating without returning data
func NoContent(c router.Context) error {
	return c.JSON(http.StatusNoContent, nil)
}

// Error sends an error response with the appropriate HTTP status code
// It uses MapError to convert application errors to HTTP responses
func Error(c router.Context, err error) error {
	statusCode, errorResponse := MapError(c.Request().Context(), err)
	return c.JSON(statusCode, errorResponse)
}
