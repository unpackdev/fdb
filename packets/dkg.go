package packets

import (
	"bytes"
	"encoding/binary"
	"go.dedis.ch/kyber/v4"
	dkg "go.dedis.ch/kyber/v4/share/dkg/pedersen"
)

// SerializeResponseBundle converts a ResponseBundle into a compact binary format.
func SerializeResponseBundle(rb *dkg.ResponseBundle) ([]byte, error) {
	buf := bytes.Buffer{}

	// Write ShareIndex as uint32
	if err := binary.Write(&buf, binary.BigEndian, rb.ShareIndex); err != nil {
		return nil, err
	}

	// Write number of Responses as uint64
	if err := binary.Write(&buf, binary.BigEndian, uint64(len(rb.Responses))); err != nil {
		return nil, err
	}

	for _, resp := range rb.Responses {
		// Write DealerIndex as uint32
		if err := binary.Write(&buf, binary.BigEndian, resp.DealerIndex); err != nil {
			return nil, err
		}

		var statusByte byte
		if err := buf.WriteByte(statusByte); err != nil {
			return nil, err
		}
	}

	// Write SessionID length and bytes
	if err := writeBytes(&buf, rb.SessionID); err != nil {
		return nil, err
	}

	// Write Signature length and bytes
	if err := writeBytes(&buf, rb.Signature); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeserializeResponseBundle parses binary data into a ResponseBundle.
func DeserializeResponseBundle(data []byte) (*dkg.ResponseBundle, error) {
	buf := bytes.NewReader(data)
	rb := &dkg.ResponseBundle{}

	// Read ShareIndex
	if err := binary.Read(buf, binary.BigEndian, &rb.ShareIndex); err != nil {
		return nil, err
	}

	// Read number of Responses
	var numResponses uint64
	if err := binary.Read(buf, binary.BigEndian, &numResponses); err != nil {
		return nil, err
	}

	for i := uint64(0); i < numResponses; i++ {
		var dealerIndex uint32
		if err := binary.Read(buf, binary.BigEndian, &dealerIndex); err != nil {
			return nil, err
		}

		statusByte, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}

		rb.Responses = append(rb.Responses, dkg.Response{
			DealerIndex: dealerIndex,
			Status:      dkg.Status(statusByte),
		})
	}

	// Read SessionID
	sessionID, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	rb.SessionID = sessionID

	// Read Signature
	signature, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	rb.Signature = signature

	return rb, nil
}

// SerializeJustificationBundle converts a JustificationBundle into a compact binary format.
func SerializeJustificationBundle(jb *dkg.JustificationBundle) ([]byte, error) {
	buf := bytes.Buffer{}

	// Write DealerIndex as uint32
	if err := binary.Write(&buf, binary.BigEndian, jb.DealerIndex); err != nil {
		return nil, err
	}

	// Write number of Justifications as uint64
	if err := binary.Write(&buf, binary.BigEndian, uint64(len(jb.Justifications))); err != nil {
		return nil, err
	}

	for _, just := range jb.Justifications {
		// Write ShareIndex as uint32
		if err := binary.Write(&buf, binary.BigEndian, just.ShareIndex); err != nil {
			return nil, err
		}

		// Write Share length and bytes
		shareBytes, err := just.Share.MarshalBinary()
		if err != nil {
			return nil, err
		}
		if err := writeBytes(&buf, shareBytes); err != nil {
			return nil, err
		}
	}

	// Write SessionID length and bytes
	if err := writeBytes(&buf, jb.SessionID); err != nil {
		return nil, err
	}

	// Write Signature length and bytes
	if err := writeBytes(&buf, jb.Signature); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeserializeJustificationBundle parses binary data into a JustificationBundle.
func DeserializeJustificationBundle(data []byte) (*dkg.JustificationBundle, error) {
	buf := bytes.NewReader(data)
	jb := &dkg.JustificationBundle{}

	// Read DealerIndex
	if err := binary.Read(buf, binary.BigEndian, &jb.DealerIndex); err != nil {
		return nil, err
	}

	// Read number of Justifications
	var numJustifications uint64
	if err := binary.Read(buf, binary.BigEndian, &numJustifications); err != nil {
		return nil, err
	}

	for i := uint64(0); i < numJustifications; i++ {
		var shareIndex uint32
		if err := binary.Read(buf, binary.BigEndian, &shareIndex); err != nil {
			return nil, err
		}

		shareBytes, err := readBytes(buf)
		if err != nil {
			return nil, err
		}

		var share kyber.Scalar
		// Initialize the Scalar using the appropriate suite
		share = jb.Justifications[0].Share
		if err := share.UnmarshalBinary(shareBytes); err != nil {
			return nil, err
		}

		jb.Justifications = append(jb.Justifications, dkg.Justification{
			ShareIndex: shareIndex,
			Share:      share,
		})
	}

	// Read SessionID
	sessionID, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	jb.SessionID = sessionID

	// Read Signature
	signature, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	jb.Signature = signature

	return jb, nil
}

// SerializeDealBundle converts a DealBundle into a compact binary format.
func SerializeDealBundle(db *dkg.DealBundle) ([]byte, error) {
	buf := bytes.Buffer{}

	// Write DealerIndex as uint32
	if err := binary.Write(&buf, binary.BigEndian, db.DealerIndex); err != nil {
		return nil, err
	}

	// Write number of Deals as uint64
	if err := binary.Write(&buf, binary.BigEndian, uint64(len(db.Deals))); err != nil {
		return nil, err
	}

	for _, deal := range db.Deals {
		// Write ShareIndex as uint32
		if err := binary.Write(&buf, binary.BigEndian, deal.ShareIndex); err != nil {
			return nil, err
		}

		// Write EncryptedShare length and bytes
		if err := writeBytes(&buf, deal.EncryptedShare); err != nil {
			return nil, err
		}
	}

	// Write number of Public points as uint64
	if err := binary.Write(&buf, binary.BigEndian, uint64(len(db.Public))); err != nil {
		return nil, err
	}

	for _, pub := range db.Public {
		pubBytes, err := pub.MarshalBinary()
		if err != nil {
			return nil, err
		}
		if err := writeBytes(&buf, pubBytes); err != nil {
			return nil, err
		}
	}

	// Write SessionID length and bytes
	if err := writeBytes(&buf, db.SessionID); err != nil {
		return nil, err
	}

	// Write Signature length and bytes
	if err := writeBytes(&buf, db.Signature); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeserializeDealBundle parses binary data into a DealBundle.
func DeserializeDealBundle(suite dkg.Suite, data []byte) (*dkg.DealBundle, error) {
	buf := bytes.NewReader(data)
	db := &dkg.DealBundle{}

	// Read DealerIndex
	if err := binary.Read(buf, binary.BigEndian, &db.DealerIndex); err != nil {
		return nil, err
	}

	// Read number of Deals
	var numDeals uint64
	if err := binary.Read(buf, binary.BigEndian, &numDeals); err != nil {
		return nil, err
	}

	for i := uint64(0); i < numDeals; i++ {
		var shareIndex uint32
		if err := binary.Read(buf, binary.BigEndian, &shareIndex); err != nil {
			return nil, err
		}

		encryptedShare, err := readBytes(buf)
		if err != nil {
			return nil, err
		}

		db.Deals = append(db.Deals, dkg.Deal{
			ShareIndex:     shareIndex,
			EncryptedShare: encryptedShare,
		})
	}

	// Read number of Public points
	var numPubs uint64
	if err := binary.Read(buf, binary.BigEndian, &numPubs); err != nil {
		return nil, err
	}

	for i := uint64(0); i < numPubs; i++ {
		pubBytes, err := readBytes(buf)
		if err != nil {
			return nil, err
		}

		var pub = suite.Point()
		if pErr := pub.UnmarshalBinary(pubBytes); pErr != nil {
			return nil, pErr
		}
		db.Public = append(db.Public, pub)
	}

	// Read SessionID
	sessionID, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	db.SessionID = sessionID

	// Read Signature
	signature, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	db.Signature = signature

	return db, nil
}
