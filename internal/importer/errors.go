package importer

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return strings.Join(e.Problems, "; ")
}

func validationf(format string, args ...interface{}) error {
	return &ValidationError{Problems: []string{fmt.Sprintf(format, args...)}}
}

func appendProblem(problems *[]string, line int, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if line > 0 {
		message = fmt.Sprintf("line %d: %s", line, message)
	}
	*problems = append(*problems, message)
}
