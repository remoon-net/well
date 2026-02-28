package firewall

import "github.com/pocketbase/pocketbase/tools/store"

type Allows = store.Store[uint16, empty]

type ProtoAllows struct {
	TCP *Allows
	UDP *Allows
}

func NewProtoAllows() *ProtoAllows {
	return &ProtoAllows{
		TCP: store.New(map[uint16]empty{}),
		UDP: store.New(map[uint16]empty{}),
	}
}

func (pa *ProtoAllows) Reset() {
	pa.TCP.Reset(nil)
	pa.UDP.Reset(nil)
}
