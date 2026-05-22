package sqlite

import "fmt"

func wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("sqlite store: %s: %w", op, err)
}
