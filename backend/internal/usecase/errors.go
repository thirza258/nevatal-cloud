package usecase

// ErrorKind classifies a business error so the API layer can pick the HTTP
// status without knowing the business rule behind it.
type ErrorKind int

const (
	KindInvalid         ErrorKind = iota + 1 // malformed or unacceptable input
	KindUnauthorized                         // missing or wrong credentials
	KindForbidden                            // authenticated but not allowed
	KindNotFound                             // resource absent or owned by another tenant
	KindConflict                             // state does not allow the action
	KindUnprocessable                        // well-formed input that cannot be applied
	KindUnavailable                          // feature disabled or not configured
	KindBadGateway                           // an upstream service failed
	KindTooManyRequests                      // a per-user limit is reached
)

// Error is a business error with a stable, client-visible code. Use cases
// return it when an endpoint answers different conditions with different
// status codes; other errors are reported with the endpoint's default code.
type Error struct {
	Kind    ErrorKind
	Code    string
	Message string
	// Field optionally names the request field the error is about; the API
	// reports it as details.field.
	Field string
	// Details, when set, is reported as the error's details object (Field
	// is then ignored).
	Details any
}

func (e *Error) Error() string { return e.Message }

func newError(kind ErrorKind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// ValidationError is a 400 VALIDATION_ERROR about one request field.
func ValidationError(field, message string) *Error {
	return &Error{Kind: KindInvalid, Code: "VALIDATION_ERROR", Message: message, Field: field}
}
