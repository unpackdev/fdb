package capn

import (
	"capnproto.org/go/capnp/v3"
	"fmt"
	"github.com/unpackdev/fdb/protocols/capn/schema"
)

// CreateGetRequest creates a new Cap'n Proto serialized Get request
func CreateGetRequest(key []byte) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root request object
	request, err := schema.NewRootRequest(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create the get request
	request.SetGet()
	get := request.Get()

	// Set the key
	if err := get.SetKey(key); err != nil {
		return nil, fmt.Errorf("failed to set key: %w", err)
	}

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// CreateSetRequest creates a new Cap'n Proto serialized Set request
func CreateSetRequest(key, value []byte) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root request object
	request, err := schema.NewRootRequest(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create the set request
	request.SetSet()
	set := request.Set()

	// Set the key and value
	if err := set.SetKey(key); err != nil {
		return nil, fmt.Errorf("failed to set key: %w", err)
	}
	if err := set.SetValue(value); err != nil {
		return nil, fmt.Errorf("failed to set value: %w", err)
	}

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// CreateDeleteRequest creates a new Cap'n Proto serialized Delete request
func CreateDeleteRequest(key []byte) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root request object
	request, err := schema.NewRootRequest(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create the delete request
	request.SetDelete()
	del := request.Delete()

	// Set the key
	if err := del.SetKey(key); err != nil {
		return nil, fmt.Errorf("failed to set key: %w", err)
	}

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// CreateExistsRequest creates a new Cap'n Proto serialized Exists request
func CreateExistsRequest(key []byte) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root request object
	request, err := schema.NewRootRequest(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create the exists request
	request.SetExists()
	exists := request.Exists()

	// Set the key
	if err := exists.SetKey(key); err != nil {
		return nil, fmt.Errorf("failed to set key: %w", err)
	}

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// CreateErrorResponse creates a new Cap'n Proto serialized Error response
func CreateErrorResponse(errorMsg string) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root response object
	response, err := schema.NewRootResponse(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	// Create the error response
	response.SetError()
	errResp := response.Error()

	// Set the error message
	if err := errResp.SetMessage_(errorMsg); err != nil {
		return nil, fmt.Errorf("failed to set error message: %w", err)
	}

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// ParseResponse parses a Cap'n Proto serialized response
func ParseResponse(data []byte) (schema.Response, error) {
	// Unmarshal the message
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return schema.Response{}, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Read the root response object
	return schema.ReadRootResponse(msg)
}

// CreateGetResponse creates a new Cap'n Proto serialized Get response
func CreateGetResponse(value []byte, exists bool) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root response object
	response, err := schema.NewRootResponse(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	// Create the get response
	response.SetGet()
	get := response.Get()

	// Set the value and exists flag
	if value != nil {
		if err := get.SetValue(value); err != nil {
			return nil, fmt.Errorf("failed to set value: %w", err)
		}
	}
	get.SetExists(exists)

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// ParseGetResponse parses a Cap'n Proto Get response
func ParseGetResponse(response schema.Response) ([]byte, bool, error) {
	if response.Which() != schema.Response_Which_get {
		return nil, false, fmt.Errorf("unexpected response type: %s", response.Which())
	}

	get := response.Get()
	exists := get.Exists()

	var value []byte
	var err error
	if exists {
		value, err = get.Value()
		if err != nil {
			return nil, false, fmt.Errorf("failed to get value: %w", err)
		}
	}

	return value, exists, nil
}

// CreateSetResponse creates a new Cap'n Proto serialized Set response
func CreateSetResponse(success bool) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root response object
	response, err := schema.NewRootResponse(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	// Create the set response
	response.SetSet()
	set := response.Set()

	// Set the success flag
	set.SetSuccess(success)

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// ParseSetResponse parses a Cap'n Proto Set response
func ParseSetResponse(response schema.Response) (bool, error) {
	if response.Which() != schema.Response_Which_set {
		return false, fmt.Errorf("unexpected response type: %s", response.Which())
	}

	set := response.Set()
	return set.Success(), nil
}

// CreateDeleteResponse creates a new Cap'n Proto serialized Delete response
func CreateDeleteResponse(success bool) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root response object
	response, err := schema.NewRootResponse(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	// Create the delete response
	response.SetDelete()
	del := response.Delete()

	// Set the success flag
	del.SetSuccess(success)

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// ParseDeleteResponse parses a Cap'n Proto Delete response
func ParseDeleteResponse(response schema.Response) (bool, error) {
	if response.Which() != schema.Response_Which_delete {
		return false, fmt.Errorf("unexpected response type: %s", response.Which())
	}

	del := response.Delete()
	return del.Success(), nil
}

// CreateExistsResponse creates a new Cap'n Proto serialized Exists response
func CreateExistsResponse(exists bool) ([]byte, error) {
	// Create a new empty message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Create the root response object
	response, err := schema.NewRootResponse(seg)
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	// Create the exists response
	response.SetExists()
	existsResp := response.Exists()

	// Set the exists flag
	existsResp.SetExists(exists)

	// Marshal the message
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	return data, nil
}

// ParseExistsResponse parses a Cap'n Proto Exists response
func ParseExistsResponse(response schema.Response) (bool, error) {
	if response.Which() != schema.Response_Which_exists {
		return false, fmt.Errorf("unexpected response type: %s", response.Which())
	}

	exists := response.Exists()
	return exists.Exists(), nil
}
