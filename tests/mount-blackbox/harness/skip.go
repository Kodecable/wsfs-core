package harness

import "errors"

type skipError struct {
	reason string
}

func (e skipError) Error() string {
	if e.reason == "" {
		return "skipped"
	}
	return e.reason
}

func Skip(reason string) error {
	return skipError{reason: reason}
}

func IsSkip(err error) bool {
	var target skipError
	return errors.As(err, &target)
}
