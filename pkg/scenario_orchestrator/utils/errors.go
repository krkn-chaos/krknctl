package utils

import "fmt"

type ExitError struct {
	ExitStatus int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("Krkn exited with exit status: %d", e.ExitStatus)
}
