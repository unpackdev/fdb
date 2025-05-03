// pkg/packets/actor_packet_test.go

package packets

//
//import (
//	"github.com/peerdns/peerd/pkg/types"
//	"testing"
//
//	"github.com/libp2p/go-libp2p/core/peer"
//
//	"github.com/stretchr/testify/assert"
//)
//
//var (
//	TestRoleOne types.Role = "one"
//	TestRoleTwo types.Role = "two"
//)
//
//func TestActorResponsePacketSerialization(t *testing.T) {
//	// Create a sample ActorResponsePacket
//	arp := &ActorResponsePacket{
//		Status:  1,
//		Message: "Approved actor information.",
//		Roles: types.Roles{
//			TestRoleOne,
//			TestRoleTwo,
//		},
//		SupportedTransports: []types.TransportType{
//			types.TCPTransport,
//			types.QUICTransport,
//		},
//		SupportedProtocols: []types.ProtocolType{
//			types.HTTPProtocol,
//			types.RPCProtocol,
//		},
//		SupportedSigners: []types.SignerType{
//			types.BlsSignerType,
//			types.Ed25519SignerType,
//		},
//	}
//
//	// Serialize the packet
//	serialized, err := arp.Serialize()
//	assert.NoError(t, err, "Serialization should not produce an error")
//
//	// Deserialize the packet
//	deserialized, err := DeserializeActorResponsePacket(serialized)
//	assert.NoError(t, err, "Deserialization should not produce an error")
//
//	// Compare original and deserialized packets
//	assert.Equal(t, arp.Status, deserialized.Status, "Status should match")
//	assert.Equal(t, arp.Message, deserialized.Message, "Message should match")
//	assert.Equal(t, arp.Roles, deserialized.Roles, "Roles should match")
//	assert.Equal(t, arp.SupportedTransports, deserialized.SupportedTransports, "SupportedTransports should match")
//	assert.Equal(t, arp.SupportedProtocols, deserialized.SupportedProtocols, "SupportedProtocols should match")
//	assert.Equal(t, arp.SupportedSigners, deserialized.SupportedSigners, "SupportedSigners should match")
//}
//
//func TestNetworkPacketWithActorResponse(t *testing.T) {
//	// Create a sample ActorResponsePacket
//	arp := &ActorResponsePacket{
//		Status:  1,
//		Message: "Approved actor information.",
//		Roles: types.Roles{
//			TestRoleOne,
//			TestRoleTwo,
//		},
//		SupportedTransports: []types.TransportType{
//			types.TCPTransport,
//			types.QUICTransport,
//		},
//		SupportedProtocols: []types.ProtocolType{
//			types.HTTPProtocol,
//			types.RPCProtocol,
//		},
//		SupportedSigners: []types.SignerType{
//			types.BlsSignerType,
//			types.Ed25519SignerType,
//		},
//	}
//
//	// Serialize ActorResponsePacket
//	payload, err := arp.Serialize()
//	assert.NoError(t, err, "Failed to serialize ActorResponsePacket")
//
//	// Create a NetworkPacket
//	senderID, _ := peer.Decode("QmSenderID1234567890")
//	receiverID, _ := peer.Decode("QmReceiverID0987654321")
//	networkPacket := &NetworkPacket{
//		Type:       ActorResponseType,
//		SenderID:   senderID,
//		ReceiverID: receiverID,
//		Payload:    payload,
//		Signature:  []byte{0x01, 0x02, 0x03}, // Sample signature
//	}
//
//	// Serialize NetworkPacket
//	serializedNetworkPacket, err := networkPacket.Serialize()
//	assert.NoError(t, err, "Failed to serialize NetworkPacket")
//
//	// Deserialize NetworkPacket
//	deserializedNetworkPacket, err := DeserializeNetworkPacket(serializedNetworkPacket)
//	assert.NoError(t, err, "Failed to deserialize NetworkPacket")
//	assert.Equal(t, networkPacket.Type, deserializedNetworkPacket.Type, "PacketType should match")
//	assert.Equal(t, networkPacket.SenderID, deserializedNetworkPacket.SenderID, "SenderID should match")
//	assert.Equal(t, networkPacket.ReceiverID, deserializedNetworkPacket.ReceiverID, "ReceiverID should match")
//	assert.Equal(t, networkPacket.Payload, deserializedNetworkPacket.Payload, "Payload should match")
//	assert.Equal(t, networkPacket.Signature, deserializedNetworkPacket.Signature, "Signature should match")
//
//	// Deserialize ActorResponsePacket from payload
//	deserializedARP, err := DeserializeActorResponsePacket(deserializedNetworkPacket.Payload)
//	assert.NoError(t, err, "Failed to deserialize ActorResponsePacket")
//	assert.Equal(t, arp.Status, deserializedARP.Status, "Status should match")
//	assert.Equal(t, arp.Message, deserializedARP.Message, "Message should match")
//	assert.Equal(t, arp.Roles, deserializedARP.Roles, "Roles should match")
//	assert.Equal(t, arp.SupportedTransports, deserializedARP.SupportedTransports, "SupportedTransports should match")
//	assert.Equal(t, arp.SupportedProtocols, deserializedARP.SupportedProtocols, "SupportedProtocols should match")
//	assert.Equal(t, arp.SupportedSigners, deserializedARP.SupportedSigners, "SupportedSigners should match")
//}
