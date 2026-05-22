package postgres

import "fmt"

func wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("postgres store: %s: %w", op, err)
}

func wrapSecretErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("postgres store secrets %s: %w", op, err)
}
