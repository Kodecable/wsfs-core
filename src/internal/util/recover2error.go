package util

import "fmt"

func RecoverValue(obj any) error {
	if obj == nil {
		return nil
	}
	if err, ok := obj.(error); ok {
		return err
	} else {
		return fmt.Errorf("recover got: %v", obj)
	}
}
