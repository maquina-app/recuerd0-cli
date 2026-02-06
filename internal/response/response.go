package response

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/maquina/recuerd0-cli/internal/errors"
)

var prettyPrint bool

// SetPrettyPrint enables or disables indented JSON output.
func SetPrettyPrint(enabled bool) {
	prettyPrint = enabled
}

// Pagination holds pagination state for list responses.
type Pagination struct {
	HasNext bool   `json:"has_next"`
	NextURL string `json:"next_url,omitempty"`
}

// Breadcrumb suggests a next action for AI tool consumption.
type Breadcrumb struct {
	Action      string `json:"action"`
	Cmd         string `json:"cmd"`
	Description string `json:"description"`
}

// ErrorDetail holds error information in the JSON envelope.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status,omitempty"`
}

// Response is the JSON envelope for all CLI output.
type Response struct {
	Success     bool                   `json:"success"`
	Data        interface{}            `json:"data,omitempty"`
	Error       *ErrorDetail           `json:"error,omitempty"`
	Pagination  *Pagination            `json:"pagination,omitempty"`
	Breadcrumbs []Breadcrumb           `json:"breadcrumbs,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	Location    string                 `json:"location,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

func newMeta() map[string]interface{} {
	return map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
}

// Success creates a basic success response.
func Success(data interface{}) *Response {
	return &Response{
		Success: true,
		Data:    data,
		Meta:    newMeta(),
	}
}

// SuccessWithSummary creates a success response with a human-readable summary.
func SuccessWithSummary(data interface{}, summary string) *Response {
	return &Response{
		Success: true,
		Data:    data,
		Summary: summary,
		Meta:    newMeta(),
	}
}

// SuccessWithLocation creates a success response with a location header (201 Created).
func SuccessWithLocation(data interface{}, location string) *Response {
	return &Response{
		Success:  true,
		Data:     data,
		Location: location,
		Meta:     newMeta(),
	}
}

// SuccessWithPagination creates a success response with pagination info.
func SuccessWithPagination(data interface{}, hasNext bool, nextURL string) *Response {
	return &Response{
		Success: true,
		Data:    data,
		Pagination: &Pagination{
			HasNext: hasNext,
			NextURL: nextURL,
		},
		Meta: newMeta(),
	}
}

// SuccessWithBreadcrumbs creates a success response with breadcrumbs and summary.
func SuccessWithBreadcrumbs(data interface{}, summary string, breadcrumbs []Breadcrumb) *Response {
	return &Response{
		Success:     true,
		Data:        data,
		Summary:     summary,
		Breadcrumbs: breadcrumbs,
		Meta:        newMeta(),
	}
}

// SuccessWithPaginationAndBreadcrumbs creates the full response with all fields.
func SuccessWithPaginationAndBreadcrumbs(data interface{}, hasNext bool, nextURL string, summary string, breadcrumbs []Breadcrumb) *Response {
	return &Response{
		Success: true,
		Data:    data,
		Pagination: &Pagination{
			HasNext: hasNext,
			NextURL: nextURL,
		},
		Summary:     summary,
		Breadcrumbs: breadcrumbs,
		Meta:        newMeta(),
	}
}

// Error creates an error response from a CLIError.
func Error(err *errors.CLIError) *Response {
	return &Response{
		Success: false,
		Error: &ErrorDetail{
			Code:    err.Code,
			Message: err.Message,
			Status:  err.Status,
		},
		Meta: newMeta(),
	}
}

// JSON serializes the response to JSON bytes.
func (r *Response) JSON() ([]byte, error) {
	if prettyPrint {
		return json.MarshalIndent(r, "", "  ")
	}
	return json.Marshal(r)
}

// Print writes the JSON response to stdout.
func (r *Response) Print() {
	data, err := r.JSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling response: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// PrintAndExit writes the JSON response to stdout and exits with the appropriate code.
func (r *Response) PrintAndExit() {
	r.Print()
	if r.Success {
		os.Exit(errors.ExitSuccess)
	}
	if r.Error != nil {
		// Map error code to exit code
		exitCode := errorCodeToExitCode(r.Error.Code)
		os.Exit(exitCode)
	}
	os.Exit(errors.ExitError)
}

func errorCodeToExitCode(code string) int {
	switch code {
	case errors.CodeInvalidArgs:
		return errors.ExitInvalidArgs
	case errors.CodeAuth:
		return errors.ExitAuthFailure
	case errors.CodeForbidden:
		return errors.ExitForbidden
	case errors.CodeNotFound:
		return errors.ExitNotFound
	case errors.CodeValidation:
		return errors.ExitValidation
	case errors.CodeNetwork:
		return errors.ExitNetwork
	case errors.CodeRateLimited:
		return errors.ExitRateLimited
	default:
		return errors.ExitError
	}
}
