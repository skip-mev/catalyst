package relayer

import (
	"testing"

	relayerv2 "github.com/cosmos/ibc/cli/api/v2/relayer"
	"github.com/stretchr/testify/require"
)

func TestRequireSelectedPackets(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		require.ErrorContains(t, requireSelectedPackets(nil), "empty relay response")
	})

	t.Run("empty packets", func(t *testing.T) {
		err := requireSelectedPackets(&relayerv2.RelayResponse{})
		require.ErrorContains(t, err, "relayer selected no packets for delivery (0 observed)")
	})

	t.Run("all unconfigured", func(t *testing.T) {
		err := requireSelectedPackets(&relayerv2.RelayResponse{
			Packets: []*relayerv2.ObservedPacket{{
				SourceClientId: "client-0",
				SequenceNumber: 1,
				Selection:      relayerv2.PacketSelection_PACKET_SELECTION_UNCONFIGURED,
			}},
		})
		require.ErrorContains(t, err, "relayer selected no packets for delivery (1 observed)")
	})

	t.Run("mixed with one selected", func(t *testing.T) {
		err := requireSelectedPackets(&relayerv2.RelayResponse{
			Packets: []*relayerv2.ObservedPacket{
				{
					SourceClientId: "a-0",
					SequenceNumber: 1,
					Selection:      relayerv2.PacketSelection_PACKET_SELECTION_UNCONFIGURED,
				},
				{
					SourceClientId: "b-0",
					SequenceNumber: 2,
					Selection:      relayerv2.PacketSelection_PACKET_SELECTION_SELECTED,
				},
			},
		})
		require.NoError(t, err)
	})
}
