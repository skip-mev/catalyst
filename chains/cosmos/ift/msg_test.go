package ift

import (
	"testing"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/stretchr/testify/require"
)

func TestMsgIFTTransferPacksWithConfiguredTypeURL(t *testing.T) {
	const typeURL = "example.ift.v1.MsgIFTTransfer"

	RegisterTypeURL(typeURL)

	msg := &MsgIFTTransfer{
		Signer:           "cosmos1deadbeefdeadbeefdeadbeefdeadbeef00",
		Denom:            "stake",
		ClientID:         "client-0",
		Receiver:         "0x1234567890123456789012345678901234567890",
		Amount:           "100",
		TimeoutTimestamp: 123,
	}

	anyMsg, err := codectypes.NewAnyWithValue(msg)
	require.NoError(t, err)
	require.Equal(t, "/"+typeURL, anyMsg.TypeUrl)
}

func TestMsgIFTTransferPacksWithConfiguredTypeURLLeadingSlash(t *testing.T) {
	const typeURL = "/example.ift.v1.MsgIFTTransfer"

	RegisterTypeURL(typeURL)

	msg := &MsgIFTTransfer{
		Signer:           "cosmos1deadbeefdeadbeefdeadbeefdeadbeef00",
		Denom:            "stake",
		ClientID:         "client-0",
		Receiver:         "0x1234567890123456789012345678901234567890",
		Amount:           "100",
		TimeoutTimestamp: 123,
	}

	anyMsg, err := codectypes.NewAnyWithValue(msg)
	require.NoError(t, err)
	require.Equal(t, "/example.ift.v1.MsgIFTTransfer", anyMsg.TypeUrl)
}
