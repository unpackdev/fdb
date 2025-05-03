// pkg/protocols/rpc/marshaller.go
package rpc

import "github.com/goccy/go-json"

// MarshalRequest marshals a Request into JSON.
func MarshalRequest(req Request) ([]byte, error) {
	return json.Marshal(req)
}

// UnmarshalResponse unmarshals JSON data into a Response.
func UnmarshalResponse(data []byte, resp *Response) error {
	return json.Unmarshal(data, resp)
}
