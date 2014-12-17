package message

import (
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	blocks "github.com/jbenet/go-ipfs/blocks"
	pb "github.com/jbenet/go-ipfs/exchange/bitswap/message/internal/pb"
	wantlist "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	nm "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO move message.go into the bitswap package
// TODO move bs/msg/internal/pb to bs/internal/pb and rename pb package to bitswap_pb

type BitSwapMessage interface {
	// Wantlist returns a slice of unique keys that represent data wanted by
	// the sender.
	Wantlist() []*Entry

	// Blocks returns a slice of unique blocks
	Blocks() []*blocks.Block

	// AddEntry adds an entry to the Wantlist.
	AddEntry(key u.Key, priority int)

	Cancel(key u.Key)

	// Sets whether or not the contained wantlist represents the entire wantlist
	// true = full wantlist
	// false = wantlist 'patch'
	// default: true
	SetFull(isFull bool)

	Full() bool

	AddBlock(*blocks.Block)
	Exportable
}

type Exportable interface {
	ToProto() *pb.Message
	ToNet(p peer.Peer) (nm.NetMessage, error)
}

type impl struct {
	full     bool
	wantlist map[u.Key]*Entry
	blocks   map[u.Key]*blocks.Block // map to detect duplicates
}

func New() BitSwapMessage {
	return newMsg()
}

func newMsg() *impl {
	return &impl{
		blocks:   make(map[u.Key]*blocks.Block),
		wantlist: make(map[u.Key]*Entry),
		full:     true,
	}
}

type Entry struct {
	wantlist.Entry
	Cancel bool
}

func newMessageFromProto(pbm pb.Message) BitSwapMessage {
	m := newMsg()
	m.SetFull(pbm.GetWantlist().GetFull())
	for _, e := range pbm.GetWantlist().GetEntries() {
		m.addEntry(u.Key(e.GetBlock()), int(e.GetPriority()), e.GetCancel())
	}
	for _, d := range pbm.GetBlocks() {
		b := blocks.NewBlock(d)
		m.AddBlock(b)
	}
	return m
}

func (m *impl) SetFull(full bool) {
	m.full = full
}

func (m *impl) Full() bool {
	return m.full
}

func (m *impl) Wantlist() []*Entry {
	var out []*Entry
	for _, e := range m.wantlist {
		out = append(out, e)
	}
	return out
}

func (m *impl) Blocks() []*blocks.Block {
	bs := make([]*blocks.Block, 0)
	for _, block := range m.blocks {
		bs = append(bs, block)
	}
	return bs
}

func (m *impl) Cancel(k u.Key) {
	m.addEntry(k, 0, true)
}

func (m *impl) AddEntry(k u.Key, priority int) {
	m.addEntry(k, priority, false)
}

func (m *impl) addEntry(k u.Key, priority int, cancel bool) {
	e, exists := m.wantlist[k]
	if exists {
		e.Priority = priority
		e.Cancel = cancel
	} else {
		m.wantlist[k] = &Entry{
			Entry: wantlist.Entry{
				Key:      k,
				Priority: priority,
			},
			Cancel: cancel,
		}
	}
}

func (m *impl) AddBlock(b *blocks.Block) {
	m.blocks[b.Key()] = b
}

func FromNet(nmsg netmsg.NetMessage) (BitSwapMessage, error) {
	pb := new(pb.Message)
	if err := proto.Unmarshal(nmsg.Data(), pb); err != nil {
		return nil, err
	}
	m := newMessageFromProto(*pb)
	return m, nil
}

func (m *impl) ToProto() *pb.Message {
	pbm := new(pb.Message)
	pbm.Wantlist = new(pb.Message_Wantlist)
	for _, e := range m.wantlist {
		pbm.Wantlist.Entries = append(pbm.Wantlist.Entries, &pb.Message_Wantlist_Entry{
			Block:    proto.String(string(e.Key)),
			Priority: proto.Int32(int32(e.Priority)),
			Cancel:   &e.Cancel,
		})
	}
	for _, b := range m.Blocks() {
		pbm.Blocks = append(pbm.Blocks, b.Data)
	}
	return pbm
}

func (m *impl) ToNet(p peer.Peer) (nm.NetMessage, error) {
	return nm.FromObject(p, m.ToProto())
}
