package commands

import "github.com/maquina/recuerd0-cli/internal/client"

// MockCall records a call made to the mock client.
type MockCall struct {
	Path string
	Body interface{}
}

// MockClient implements client.API for testing.
type MockClient struct {
	GetResponse    *client.APIResponse
	PostResponse   *client.APIResponse
	PatchResponse  *client.APIResponse
	DeleteResponse *client.APIResponse

	GetError    error
	PostError   error
	PatchError  error
	DeleteError error

	GetCalls    []MockCall
	PostCalls   []MockCall
	PatchCalls  []MockCall
	DeleteCalls []MockCall
}

// NewMockClient creates a mock client with default success responses.
func NewMockClient() *MockClient {
	return &MockClient{
		GetResponse:    &client.APIResponse{StatusCode: 200},
		PostResponse:   &client.APIResponse{StatusCode: 201},
		PatchResponse:  &client.APIResponse{StatusCode: 200},
		DeleteResponse: &client.APIResponse{StatusCode: 204},
	}
}

func (m *MockClient) Get(path string) (*client.APIResponse, error) {
	m.GetCalls = append(m.GetCalls, MockCall{Path: path})
	if m.GetError != nil {
		return nil, m.GetError
	}
	return m.GetResponse, nil
}

func (m *MockClient) Post(path string, body interface{}) (*client.APIResponse, error) {
	m.PostCalls = append(m.PostCalls, MockCall{Path: path, Body: body})
	if m.PostError != nil {
		return nil, m.PostError
	}
	return m.PostResponse, nil
}

func (m *MockClient) Patch(path string, body interface{}) (*client.APIResponse, error) {
	m.PatchCalls = append(m.PatchCalls, MockCall{Path: path, Body: body})
	if m.PatchError != nil {
		return nil, m.PatchError
	}
	return m.PatchResponse, nil
}

func (m *MockClient) Delete(path string) (*client.APIResponse, error) {
	m.DeleteCalls = append(m.DeleteCalls, MockCall{Path: path})
	if m.DeleteError != nil {
		return nil, m.DeleteError
	}
	return m.DeleteResponse, nil
}

func (m *MockClient) GetWithPagination(path string) (*client.APIResponse, error) {
	return m.Get(path)
}

// WithGetData sets the Data field on the Get response.
func (m *MockClient) WithGetData(data interface{}) *MockClient {
	m.GetResponse.Data = data
	return m
}

// WithPostData sets the Data field on the Post response.
func (m *MockClient) WithPostData(data interface{}) *MockClient {
	m.PostResponse.Data = data
	return m
}

// WithPatchData sets the Data field on the Patch response.
func (m *MockClient) WithPatchData(data interface{}) *MockClient {
	m.PatchResponse.Data = data
	return m
}

// WithPostLocation sets the Location on the Post response.
func (m *MockClient) WithPostLocation(location string) *MockClient {
	m.PostResponse.Location = location
	return m
}

// WithGetLinkNext sets pagination on the Get response.
func (m *MockClient) WithGetLinkNext(nextURL string) *MockClient {
	m.GetResponse.LinkNext = nextURL
	return m
}
