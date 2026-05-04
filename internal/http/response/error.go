package response

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type HTTPErrorInfo struct {
	Status      int
	Description string
}

var errorInfoMap = map[apperror.Code]HTTPErrorInfo{
	apperror.CodeInvalidInput: {Status: fiber.StatusBadRequest, Description: "Bad request."},
	apperror.CodeUnauthorized: {Status: fiber.StatusUnauthorized, Description: "Unauthorized."},
	apperror.CodeForbidden:    {Status: fiber.StatusForbidden, Description: "Forbidden."},
	apperror.CodeNotFound:     {Status: fiber.StatusNotFound, Description: "Resource does not exist."},
	apperror.CodeConflict:     {Status: fiber.StatusConflict, Description: "Conflict."},
	apperror.CodeInternal:     {Status: fiber.StatusInternalServerError, Description: "Internal Server Error."},
	apperror.CodeUnknown:      {Status: fiber.StatusInternalServerError, Description: "Internal Server Error."},
}

var defaultHTTPErrorInfo = HTTPErrorInfo{
	Status:      fiber.StatusInternalServerError,
	Description: "Internal Server Error.",
}

func InfoOf(err error) HTTPErrorInfo {
	code := apperror.CodeOf(err)
	if info, ok := errorInfoMap[code]; ok {
		return info
	}
	return defaultHTTPErrorInfo
}

func WriteErrorResponse(c fiber.Ctx, err error, code apperror.Code) error {
	appErr := normalizeAppError(err, code)
	info := InfoOf(appErr)
	msg := strings.TrimSpace(appErr.Message())
	if msg == "" {
		msg = info.Description
	}

	body := map[string]any{
		"ErrorCode":        info.Status,
		"ErrorDescription": fmt.Sprintf("%d: %s", info.Status, msg),
		"ErrorDetails":     c.Method(),
	}
	return c.Status(info.Status).JSON(body)
}

func normalizeAppError(err error, fallbackCode apperror.Code) *apperror.Error {
	if err == nil {
		return apperror.New(fallbackCode, "")
	}

	var appErr *apperror.Error
	if errors.As(err, &appErr) && appErr != nil {
		return appErr
	}

	return apperror.New(fallbackCode, "")
}
