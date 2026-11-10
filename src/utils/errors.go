package utils

type AppError struct {
	Status  int
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(status int, message string) *AppError {
	return &AppError{Status: status, Message: message}
}

func BadRequest(message string) *AppError {
	return NewAppError(400, message)
}

func Unauthorized(message string) *AppError {
	return NewAppError(401, message)
}

func Forbidden(message string) *AppError {
	return NewAppError(403, message)
}

func NotFound(message string) *AppError {
	return NewAppError(404, message)
}

func Conflict(message string) *AppError {
	return NewAppError(409, message)
}

func Internal() *AppError {
	return NewAppError(500, "internal server error")
}
