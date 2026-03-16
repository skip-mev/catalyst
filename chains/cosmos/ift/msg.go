package ift

import (
	"fmt"
	"strings"
	"sync"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

const DefaultMsgIFTTransferTypeURL = "catalyst.ift.v1.MsgIFTTransfer"

var (
	typeRegistrationMu sync.Mutex
	registeredTypeURLs = map[string]struct{}{}
)

type MsgIFTTransfer struct {
	Signer           string `protobuf:"bytes,1,opt,name=signer,proto3" json:"signer,omitempty"`
	Denom            string `protobuf:"bytes,2,opt,name=denom,proto3" json:"denom,omitempty"`
	ClientId         string `protobuf:"bytes,3,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Receiver         string `protobuf:"bytes,4,opt,name=receiver,proto3" json:"receiver,omitempty"`
	Amount           string `protobuf:"bytes,5,opt,name=amount,proto3" json:"amount,omitempty"`
	TimeoutTimestamp uint64 `protobuf:"varint,6,opt,name=timeout_timestamp,json=timeoutTimestamp,proto3" json:"timeout_timestamp,omitempty"`
}

func (m *MsgIFTTransfer) Reset()         { *m = MsgIFTTransfer{} }
func (m *MsgIFTTransfer) String() string { return gogoproto.CompactTextString(m) }
func (*MsgIFTTransfer) ProtoMessage()    {}

func RegisterTypeURL(typeURL string) {
	if typeURL == "" {
		typeURL = DefaultMsgIFTTransferTypeURL
	}
	typeURL = strings.TrimPrefix(typeURL, "/")

	typeRegistrationMu.Lock()
	defer typeRegistrationMu.Unlock()

	if _, exists := registeredTypeURLs[typeURL]; exists {
		return
	}

	gogoproto.RegisterType((*MsgIFTTransfer)(nil), typeURL)
	registeredTypeURLs[typeURL] = struct{}{}
}

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil), &MsgIFTTransfer{})
}

func (m *MsgIFTTransfer) GetSigners() []sdk.AccAddress {
	if m.Signer == "" {
		return []sdk.AccAddress{}
	}

	return []sdk.AccAddress{sdk.MustAccAddressFromBech32(m.Signer)}
}

func (m *MsgIFTTransfer) ValidateBasic() error {
	if m.Signer == "" {
		return fmt.Errorf("signer must be specified")
	}
	if m.Denom == "" {
		return fmt.Errorf("denom must be specified")
	}
	if m.ClientId == "" {
		return fmt.Errorf("client_id must be specified")
	}
	if m.Receiver == "" {
		return fmt.Errorf("receiver must be specified")
	}
	if m.Amount == "" {
		return fmt.Errorf("amount must be specified")
	}
	if m.TimeoutTimestamp == 0 {
		return fmt.Errorf("timeout_timestamp must be specified")
	}
	return nil
}
