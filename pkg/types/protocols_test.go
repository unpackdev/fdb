// pkg/types/protocol_test.go
package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolType_String(t *testing.T) {
	tests := []struct {
		pt       ProtocolType
		expected string
	}{
		{HTTPProtocol, "http"},
		{RPCProtocol, "rpc"},
		{WebSocketProtocol, "websocket"},
		{ProtocolType(999), "unknown"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.pt.String())
	}
}

func TestParseProtocolType(t *testing.T) {
	tests := []struct {
		input       string
		expected    ProtocolType
		expectError bool
	}{
		{"http", HTTPProtocol, false},
		{"rpc", RPCProtocol, false},
		{"websocket", WebSocketProtocol, false},
		{"invalid", -1, true},
	}

	for _, test := range tests {
		pt, err := ParseProtocolType(test.input)
		if test.expectError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expected, pt)
		}
	}
}

func TestProtocolType_UnmarshalYAML(t *testing.T) {
	type Config struct {
		Protocol ProtocolType `yaml:"protocol"`
	}

	// Test valid unmarshalling
	err := (&Config{}).Protocol.UnmarshalYAML(func(v interface{}) error {
		*(v.(*string)) = "http"
		return nil
	})
	var cfg Config
	err = cfg.Protocol.UnmarshalYAML(func(v interface{}) error {
		*(v.(*string)) = "http"
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, HTTPProtocol, cfg.Protocol)

	// Test invalid unmarshalling
	err = cfg.Protocol.UnmarshalYAML(func(v interface{}) error {
		*(v.(*string)) = "invalid"
		return nil
	})
	assert.Error(t, err)
}
