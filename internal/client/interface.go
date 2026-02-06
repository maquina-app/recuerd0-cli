package client

// APIResponse holds the parsed response from the API.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Location   string
	LinkNext   string
	Data       interface{}
}

// API defines the interface for the Recuerd0 API client.
type API interface {
	Get(path string) (*APIResponse, error)
	Post(path string, body interface{}) (*APIResponse, error)
	Patch(path string, body interface{}) (*APIResponse, error)
	Delete(path string) (*APIResponse, error)
	GetWithPagination(path string) (*APIResponse, error)
}
