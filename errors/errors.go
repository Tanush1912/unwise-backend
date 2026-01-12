package errors

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeUnauthorized            ErrorCode = "AUTH_001"
	CodeTokenExpired            ErrorCode = "AUTH_002"
	CodeTokenInvalid            ErrorCode = "AUTH_003"
	CodeInsufficientPermissions ErrorCode = "AUTH_004"
	CodeNotGroupMember          ErrorCode = "AUTH_005"

	CodeInvalidRequest       ErrorCode = "VALIDATION_001"
	CodeMissingRequiredField ErrorCode = "VALIDATION_002"
	CodeInvalidFieldFormat   ErrorCode = "VALIDATION_003"
	CodeInvalidAmount        ErrorCode = "VALIDATION_004"
	CodeAmountMismatch       ErrorCode = "VALIDATION_005"
	CodeInvalidEmail         ErrorCode = "VALIDATION_006"
	CodeInvalidUUID          ErrorCode = "VALIDATION_007"

	CodeNotFound        ErrorCode = "NOT_FOUND_001"
	CodeUserNotFound    ErrorCode = "NOT_FOUND_002"
	CodeGroupNotFound   ErrorCode = "NOT_FOUND_003"
	CodeExpenseNotFound ErrorCode = "NOT_FOUND_004"
	CodeFriendNotFound  ErrorCode = "NOT_FOUND_005"

	CodeConflict         ErrorCode = "CONFLICT_001"
	CodeDuplicateEntry   ErrorCode = "CONFLICT_002"
	CodeAlreadyMember    ErrorCode = "CONFLICT_003"
	CodeAlreadyFriends   ErrorCode = "CONFLICT_004"
	CodeCannotSelfAction ErrorCode = "CONFLICT_005"

	CodeBusinessError                 ErrorCode = "BUSINESS_001"
	CodeOutstandingBalance            ErrorCode = "BUSINESS_002"
	CodeCannotDeleteWithDebts         ErrorCode = "BUSINESS_003"
	CodeCannotRemoveMemberWithBalance ErrorCode = "BUSINESS_004"
	CodeInvalidSettlement             ErrorCode = "BUSINESS_005"

	CodeDatabaseError       ErrorCode = "DATABASE_001"
	CodeDatabaseConnection  ErrorCode = "DATABASE_002"
	CodeDatabaseQuery       ErrorCode = "DATABASE_003"
	CodeDatabaseTransaction ErrorCode = "DATABASE_004"

	CodeExternalServiceError ErrorCode = "EXTERNAL_001"
	CodeStorageError         ErrorCode = "EXTERNAL_002"
	CodeAIServiceError       ErrorCode = "EXTERNAL_003"

	CodeInternalError ErrorCode = "INTERNAL_001"
)

type ErrorType int

const (
	ErrorTypeUnauthorized ErrorType = iota
	ErrorTypeForbidden
	ErrorTypeBadRequest
	ErrorTypeNotFound
	ErrorTypeConflict
	ErrorTypeUnprocessable
	ErrorTypeInternal
	ErrorTypeServiceUnavailable
)

type AppError struct {
	Type    ErrorType `json:"-"`
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Err     error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) UserMessage() string {
	return e.Message
}

func Unauthorized(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeUnauthorized,
		Code:    CodeUnauthorized,
		Message: message,
	}
}

func TokenExpired() *AppError {
	return &AppError{
		Type:    ErrorTypeUnauthorized,
		Code:    CodeTokenExpired,
		Message: "Your session has expired. Please log in again.",
	}
}

func TokenInvalid() *AppError {
	return &AppError{
		Type:    ErrorTypeUnauthorized,
		Code:    CodeTokenInvalid,
		Message: "Invalid authentication token.",
	}
}

func NotGroupMember() *AppError {
	return &AppError{
		Type:    ErrorTypeForbidden,
		Code:    CodeNotGroupMember,
		Message: "You are not a member of this group.",
	}
}

func InvalidRequest(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeBadRequest,
		Code:    CodeInvalidRequest,
		Message: message,
	}
}

func InvalidRequestWithDetails(message, details string) *AppError {
	return &AppError{
		Type:    ErrorTypeBadRequest,
		Code:    CodeInvalidRequest,
		Message: message,
		Details: details,
	}
}

func MissingRequiredField(fieldName string) *AppError {
	return &AppError{
		Type:    ErrorTypeBadRequest,
		Code:    CodeMissingRequiredField,
		Message: fmt.Sprintf("%s is required.", fieldName),
	}
}

func InvalidFieldFormat(fieldName, expectedFormat string) *AppError {
	return &AppError{
		Type:    ErrorTypeBadRequest,
		Code:    CodeInvalidFieldFormat,
		Message: fmt.Sprintf("Invalid format for %s.", fieldName),
		Details: fmt.Sprintf("Expected format: %s", expectedFormat),
	}
}

func InvalidAmount(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeBadRequest,
		Code:    CodeInvalidAmount,
		Message: message,
	}
}

func AmountMismatch(splitTotal, expectedTotal float64, splitType string) *AppError {
	return &AppError{
		Type:    ErrorTypeBadRequest,
		Code:    CodeAmountMismatch,
		Message: fmt.Sprintf("Sum of %s amounts (%.2f) does not equal total amount (%.2f).", splitType, splitTotal, expectedTotal),
	}
}

func NotFound(resourceType string) *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Code:    CodeNotFound,
		Message: fmt.Sprintf("%s not found.", resourceType),
	}
}

func UserNotFound() *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Code:    CodeUserNotFound,
		Message: "User not found.",
	}
}

func UserNotFoundByEmail(email string) *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Code:    CodeUserNotFound,
		Message: fmt.Sprintf("No user found with email '%s'.", email),
		Details: "Please check the email address or ask them to sign up first.",
	}
}

func GroupNotFound() *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Code:    CodeGroupNotFound,
		Message: "Group not found.",
	}
}

func ExpenseNotFound() *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Code:    CodeExpenseNotFound,
		Message: "Expense not found.",
	}
}

func FriendNotFound() *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Code:    CodeFriendNotFound,
		Message: "Friend not found.",
	}
}

func Conflict(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeConflict,
		Code:    CodeConflict,
		Message: message,
	}
}

func DuplicateEntry(resourceType string) *AppError {
	return &AppError{
		Type:    ErrorTypeConflict,
		Code:    CodeDuplicateEntry,
		Message: fmt.Sprintf("%s already exists.", resourceType),
	}
}

func AlreadyMember() *AppError {
	return &AppError{
		Type:    ErrorTypeConflict,
		Code:    CodeAlreadyMember,
		Message: "User is already a member of this group.",
	}
}

func AlreadyFriends() *AppError {
	return &AppError{
		Type:    ErrorTypeConflict,
		Code:    CodeAlreadyFriends,
		Message: "You are already friends with this user.",
	}
}

func CannotAddSelf(action string) *AppError {
	return &AppError{
		Type:    ErrorTypeConflict,
		Code:    CodeCannotSelfAction,
		Message: fmt.Sprintf("You cannot %s yourself.", action),
	}
}

func CannotSettleToSelf() *AppError {
	return &AppError{
		Type:    ErrorTypeBadRequest,
		Code:    CodeInvalidSettlement,
		Message: "Cannot settle payment to yourself.",
	}
}

func OutstandingBalance(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeUnprocessable,
		Code:    CodeOutstandingBalance,
		Message: message,
	}
}

func CannotDeleteGroupWithDebts() *AppError {
	return &AppError{
		Type:    ErrorTypeUnprocessable,
		Code:    CodeCannotDeleteWithDebts,
		Message: "Cannot delete group while there are outstanding balances.",
		Details: "Please settle all debts before deleting this group.",
	}
}

func CannotRemoveMemberWithBalance(balance float64) *AppError {
	return &AppError{
		Type:    ErrorTypeUnprocessable,
		Code:    CodeCannotRemoveMemberWithBalance,
		Message: fmt.Sprintf("Cannot remove member with outstanding balance of $%.2f.", balance),
		Details: "This balance must be settled first.",
	}
}

func CannotDeleteAccountWithBalance() *AppError {
	return &AppError{
		Type:    ErrorTypeUnprocessable,
		Code:    CodeOutstandingBalance,
		Message: "Cannot delete account while you have outstanding balances.",
		Details: "Please settle all debts before deleting your account.",
	}
}

func DatabaseError(operation string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeInternal,
		Code:    CodeDatabaseError,
		Message: "A database error occurred. Please try again.",
		Details: operation,
		Err:     err,
	}
}

func StorageError(operation string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeInternal,
		Code:    CodeStorageError,
		Message: "Failed to process file storage. Please try again.",
		Details: operation,
		Err:     err,
	}
}

func AIServiceError(err error) *AppError {
	return &AppError{
		Type:    ErrorTypeServiceUnavailable,
		Code:    CodeAIServiceError,
		Message: "AI service is temporarily unavailable. Please try again later.",
		Err:     err,
	}
}

func InternalError(err error) *AppError {
	return &AppError{
		Type:    ErrorTypeInternal,
		Code:    CodeInternalError,
		Message: "An unexpected error occurred. Please try again.",
		Err:     err,
	}
}

func Wrap(err error, appErr *AppError) *AppError {
	appErr.Err = err
	return appErr
}

func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

func AsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

func GetHTTPStatus(errType ErrorType) int {
	switch errType {
	case ErrorTypeUnauthorized:
		return 401
	case ErrorTypeForbidden:
		return 403
	case ErrorTypeBadRequest:
		return 400
	case ErrorTypeNotFound:
		return 404
	case ErrorTypeConflict:
		return 409
	case ErrorTypeUnprocessable:
		return 422
	case ErrorTypeServiceUnavailable:
		return 503
	default:
		return 500
	}
}

func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "no rows") || contains(errStr, "not found")
}

func IsDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "duplicate key") || contains(errStr, "unique constraint")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
