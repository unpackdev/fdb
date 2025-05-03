package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/peer"
)

// serializePeerID serializes a peer ID into the writer.
func serializePeerID(writer io.Writer, id peer.ID) error {
	peerIDBytes := []byte(id)
	if err := binary.Write(writer, binary.LittleEndian, uint32(len(peerIDBytes))); err != nil {
		return fmt.Errorf("failed to serialize peer ID length: %w", err)
	}
	if _, err := writer.Write(peerIDBytes); err != nil {
		return fmt.Errorf("failed to serialize peer ID: %w", err)
	}
	return nil
}

// deserializePeerID deserializes a peer ID from the reader.
func deserializePeerID(reader io.Reader) (peer.ID, error) {
	var peerIDLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &peerIDLen); err != nil {
		return "", fmt.Errorf("failed to deserialize peer ID length: %w", err)
	}
	peerIDBytes := make([]byte, peerIDLen)
	if _, err := io.ReadFull(reader, peerIDBytes); err != nil {
		return "", fmt.Errorf("failed to deserialize peer ID: %w", err)
	}
	return peer.ID(peerIDBytes), nil
}

// writeBytes writes a length-prefixed byte slice using varint encoding.
func writeBytes(buf *bytes.Buffer, data []byte) error {
	length := uint64(len(data))
	if err := binary.Write(buf, binary.BigEndian, length); err != nil {
		return err
	}
	if _, err := buf.Write(data); err != nil {
		return err
	}
	return nil
}

// readBytes reads a length-prefixed byte slice using varint encoding.
func readBytes(buf *bytes.Reader) ([]byte, error) {
	var length uint64
	if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read length: %v", err)
	}

	data := make([]byte, length)
	n, err := io.ReadFull(buf, data)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %v", err)
	}
	if uint64(n) != length {
		return nil, fmt.Errorf("expected %d bytes, got %d", length, n)
	}

	return data, nil
}
