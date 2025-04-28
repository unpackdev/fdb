package share

import (
	"fmt"
	"github.com/ethereum/go-verkle"
	"github.com/goccy/go-json"
)

// SerializeStateDiff serializes the StateDiff into JSON bytes.
func SerializeStateDiff(sd verkle.StateDiff) ([]byte, error) {
	if sd == nil {
		return nil, fmt.Errorf("StateDiff is nil")
	}

	data, err := json.Marshal(sd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal StateDiff to JSON: %w", err)
	}
	return data, nil
}

// DeserializeStateDiff deserializes JSON bytes into a StateDiff.
func DeserializeStateDiff(data []byte) (verkle.StateDiff, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("input data is empty")
	}

	var sd verkle.StateDiff
	err := json.Unmarshal(data, &sd)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal StateDiff from JSON: %w", err)
	}
	return sd, nil
}
