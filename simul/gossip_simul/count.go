package gossip_simul

import (
	"sync"
	"time"

	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
	"golang.org/x/xerrors"
)

/*
The count-protocol returns the number of nodes reachable in a given
timeout. To correctly wait for the whole tree, every node that receives
a message sends a message to the root before contacting its children.
As long as the root receives those messages, he knows the counting
still goes on.
*/

func init() {
	network.RegisterMessage(RumCount{})
	onet.GlobalProtocolRegister("RumorSim", NewRumorSim)
}

// ProtocolRum holds all channels. If a timeout occurs or the counting
// is done, the Count-channel receives the number of nodes reachable in
// the tree.
type ProtocolRum struct {
	*onet.TreeNodeInstance
	Replies          int
	Count            chan int
	Quit             chan bool
	timeout          time.Duration
	timeoutMu        sync.Mutex
	PrepareCountChan chan struct {
		*onet.TreeNode
		PrepareCount
	}
	CountChan    chan []CountMsg
	NodeIsUpChan chan struct {
		*onet.TreeNode
		NodeIsUp
	}
}

// PrepareCount is sent so that every node can contact the root to say
// the counting is still going on.
type PrepareCount struct {
	Timeout time.Duration
}

// NodeIsUp - if it is received by the root it will reset the counter.
type NodeIsUp struct{}

// Count sends the number of children to the parent node.
type RumCount struct {
	Children int32
}

// CountMsg is wrapper around the Count-structure
type CountMsg struct {
	*onet.TreeNode
	RumCount
}

// NewCount returns a new protocolInstance
func NewRumorSim(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	p := &ProtocolRum{
		TreeNodeInstance: n,
		Quit:             make(chan bool),
		timeout:          5 * time.Second,
	}
	p.Count = make(chan int, 1)
	t := n.Tree()
	if t == nil {
		return nil, xerrors.New("cannot find tree")
	}
	if err := p.RegisterChannelsLength(len(t.List()),
		&p.CountChan); err != nil {
		log.Error("Couldn't register channel:", err)
	}
	return p, nil
}

// Start the protocol
func (p *ProtocolRum) Start() error {
	// Send an empty message
	log.Lvl3("Starting to count")
	//p.FuncPC()
	return nil
}

// Dispatch listens for all channels and waits for a timeout in case nothing
// happens for a certain duration
func (p *ProtocolRum) Dispatch() error {
	running := true
	p.SendMyRumor()
	for running {
		log.Lvl3(p.Info(), "waiting for message for", p.Timeout())
		select {
		case <-time.After(time.Duration(p.Timeout())):
			log.Lvl3(p.Info(), "timed out while waiting for", p.Timeout())
			if p.IsRoot() {
				log.Lvl2("Didn't get all children in time:", p.Replies)
				p.Count <- p.Replies
				running = false
			}
		}
	}
	p.Done()
	return nil
}

//
func (p *ProtocolRum) SendMyRumor() {
	if p.IsRoot() {
		err := p.SendRumor(2, RumCount{
			Children: int32(1),
		})
		if err != nil {
			log.Lvl2(p.Info(), "couldn't start rumor", err)
		}
	}
}

// SetTimeout sets the new timeout
func (p *ProtocolRum) SetTimeout(t time.Duration) {
	p.timeoutMu.Lock()
	p.timeout = t
	p.timeoutMu.Unlock()
}

// Timeout returns the current timeout
func (p *ProtocolRum) Timeout() time.Duration {
	p.timeoutMu.Lock()
	defer p.timeoutMu.Unlock()
	return p.timeout
}
