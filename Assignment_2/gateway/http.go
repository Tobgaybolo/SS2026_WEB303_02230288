package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON sends v as a JSON response with the given HTTP status.
func writeJSON(w http.ResponseWriter, httpStatus int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writing response: %v", err)
	}
}

func writeError(w http.ResponseWriter, httpStatus int, code, message string) {
	writeJSON(w, httpStatus, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// decodeJSON reads the request body into dst. It only checks SYNTAX (valid JSON,
// right types, no unknown fields); business rules are validated by the services.
// On failure it writes a 400 response and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		msg := "request body is not valid JSON: " + err.Error()
		if errors.Is(err, io.EOF) {
			msg = "request body is required"
		}
		writeError(w, http.StatusBadRequest, codes.InvalidArgument.String(), msg)
		return false
	}
	// Reject anything after the first JSON value.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, codes.InvalidArgument.String(),
			"request body must contain a single JSON object")
		return false
	}
	return true
}

// httpStatusFromGRPC maps a gRPC status code to an HTTP status code.
func httpStatusFromGRPC(c codes.Code) int {
	switch c {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest // 400
	case codes.NotFound:
		return http.StatusNotFound // 404
	case codes.AlreadyExists, codes.FailedPrecondition:
		return http.StatusConflict // 409
	case codes.Unavailable:
		return http.StatusServiceUnavailable // 503
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout // 504
	default:
		return http.StatusInternalServerError // 500
	}
}

// writeGRPCError converts a gRPC error from a downstream service into an HTTP
// error response. Internal errors are logged but their details are never exposed.
func writeGRPCError(w http.ResponseWriter, err error) {
	st := status.Convert(err)
	httpStatus := httpStatusFromGRPC(st.Code())
	msg := st.Message()
	if httpStatus == http.StatusInternalServerError {
		log.Printf("downstream error: %v", err)
		msg = "internal server error"
	}
	writeError(w, httpStatus, st.Code().String(), msg)
}

// upstreamCtx bounds a downstream call so a slow service can never hang a request.
func upstreamCtx(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}