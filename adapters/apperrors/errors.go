package apperrors

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
)

// ErrorCode defines common application error codes.
// Use these to standardize user-facing API response statuses.
type ErrorCode string

const (
	CodeNotFound     ErrorCode = "NOT_FOUND"       // 404
	CodeInvalidInput ErrorCode = "INVALID_INPUT"   // 400
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"    // 401
	CodeForbidden    ErrorCode = "FORBIDDEN"       // 403
	CodeConflict     ErrorCode = "CONFLICT"        // 409
	CodeInternal     ErrorCode = "INTERNAL_SERVER" // 500
	CodeUnknown      ErrorCode = "UNKNOWN"         // 500
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
	Body    map[string]any
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// WrapError creates a new Error with an optional cause.
func WrapError(code ErrorCode, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

type HTTPErrorInfo struct {
	Status      int
	Description string
}

var errorInfoMap = map[ErrorCode]HTTPErrorInfo{
	CodeInvalidInput: {Status: fiber.StatusBadRequest, Description: "Bad request."},
	CodeUnauthorized: {Status: fiber.StatusUnauthorized, Description: "Unauthorized."},
	CodeForbidden:    {Status: fiber.StatusForbidden, Description: "Forbidden."},
	CodeNotFound:     {Status: fiber.StatusNotFound, Description: "Resource does not exist."},
	CodeConflict:     {Status: fiber.StatusConflict, Description: "Conflict."},
	CodeInternal:     {Status: fiber.StatusInternalServerError, Description: "Internal Server Error."},
	CodeUnknown:      {Status: fiber.StatusInternalServerError, Description: "Internal Server Error."},
}

var defaultHTTPErrorInfo = HTTPErrorInfo{
	Status:      fiber.StatusInternalServerError,
	Description: "Internal Server Error.",
}

func GetHTTPErrorInfo(code ErrorCode) HTTPErrorInfo {
	if info, ok := errorInfoMap[code]; ok {
		return info
	}
	return defaultHTTPErrorInfo
}
