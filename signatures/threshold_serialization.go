package signatures

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/kyber/v4/share"
)

// SerializePriShare serializes a private share into bytes.
// Format: <Index (4 bytes)> || <Scalar bytes>
func SerializePriShare(suite pairing.Suite, ps *share.PriShare) ([]byte, error) {
	if suite == nil {
		return nil, fmt.Errorf("suite is nil in SerializePriShare")
	}

	// Serialize index
	iBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(iBytes, uint32(ps.I))

	// Serialize scalar
	vBytes, err := ps.V.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal scalar: %w", err)
	}

	// Concatenate index and scalar bytes
	data := append(iBytes, vBytes...)
	return data, nil
}

// DeserializePriShare deserializes bytes into a private share.
// Expects format: <Index (4 bytes)> || <Scalar bytes>
func DeserializePriShare(suite pairing.Suite, data []byte) (*share.PriShare, error) {
	if suite == nil {
		return nil, fmt.Errorf("suite is nil in DeserializePriShare")
	}

	if len(data) < 4 {
		return nil, fmt.Errorf("data too short to contain index")
	}

	// Deserialize index
	i := binary.BigEndian.Uint32(data[:4])

	// Deserialize scalar
	v := suite.G2().Scalar()
	if err := v.UnmarshalBinary(data[4:]); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scalar: %w", err)
	}

	return &share.PriShare{
		I: i,
		V: v,
	}, nil
}

// MarshalPubPoly
func MarshalPubPoly(suite pairing.Suite, poly *share.PubPoly) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Get the commitments from the PubPoly
	_, commits := poly.Info()

	// Serialize the number of commitments
	numCommits := int32(len(commits))
	if err := binary.Write(buf, binary.BigEndian, numCommits); err != nil {
		return nil, err
	}

	// Serialize each commitment
	for _, commit := range commits {
		commitBytes, err := commit.MarshalBinary()
		if err != nil {
			return nil, err
		}
		// Serialize the length of the commitment
		if err := binary.Write(buf, binary.BigEndian, int32(len(commitBytes))); err != nil {
			return nil, err
		}
		// Write the commitment bytes
		if _, err := buf.Write(commitBytes); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// UnmarshalPubPoly
func UnmarshalPubPoly(suite pairing.Suite, data []byte) (*share.PubPoly, error) {
	buf := bytes.NewReader(data)

	// Read the number of commitments
	var numCommits int32
	if err := binary.Read(buf, binary.BigEndian, &numCommits); err != nil {
		return nil, err
	}

	commits := make([]kyber.Point, numCommits)
	for i := int32(0); i < numCommits; i++ {
		// Read the length of the commitment
		var commitLen int32
		if err := binary.Read(buf, binary.BigEndian, &commitLen); err != nil {
			return nil, err
		}
		// Read the commitment bytes
		commitBytes := make([]byte, commitLen)
		if _, err := buf.Read(commitBytes); err != nil {
			return nil, err
		}
		// Unmarshal the commitment
		commit := suite.G2().Point()
		if err := commit.UnmarshalBinary(commitBytes); err != nil {
			return nil, err
		}
		commits[i] = commit
	}

	// Reconstruct PubPoly
	pubPoly := share.NewPubPoly(suite.G2(), nil, commits)
	return pubPoly, nil
}
