package inference

import "errors"

func NewHTTPStatusError(status int, body string) error {
	return &inferenceHTTPError{status: status, body: body}
}

func HTTPStatusCode(err error) int {
	var httpErr *inferenceHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.status
	}
	return 0
}

func HTTPErrorMessage(err error) string {
	var httpErr *inferenceHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.body
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
