package clover

import (
	"context"
	"encoding/json"
	"net/http"
)

// errorResponse is our response payload for an error
type errorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// dataResponse is our payload for data responses
type dataResponse struct {
	Message string      `json:"message"`
	Data    any `json:"data"`
}

func writeDataResponse(ctx context.Context, w http.ResponseWriter, statusCode int, message string, data any) error {
	return writeJSONResponse(ctx, w, statusCode, dataResponse{message, data})
}

func writeErrorResponse(ctx context.Context, w http.ResponseWriter, statusCode int, message string, err error) error {
	errorResponse := errorResponse{
		Message: message,
		Error:   err.Error(),
	}
	return writeJSONResponse(ctx, w, statusCode, errorResponse)
}

func writeJSONResponse(ctx context.Context, w http.ResponseWriter, statusCode int, response any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(response)
}
