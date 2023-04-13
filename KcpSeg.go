package mykcp

type KCPSEG struct {
	Conv uint32
	Cmd uint32
	Frg uint32
	Wnd uint32
	Ts uint32
	Sn uint32
	Una uint32
	Len uint32
	Resendts uint32
	Rto uint32
	Fastack uint32
	Xmit uint32
	Data []byte
}

func newKcpSeg(size int) KCPSEG {
	kcpSeg := KCPSEG{}
	kcpSeg.Data = make([]byte, size)
	return kcpSeg
}

type SegQueueNode struct {
	Next *SegQueueNode
	Prev *SegQueueNode
	Seg	 *KCPSEG
}

type SegQueue struct {
	head *SegQueueNode
	tail *SegQueueNode
	len  int
	snToNodeMap map[uint32]*SegQueueNode
}

func NewSegQueue() *SegQueue {
	q := &SegQueue{}
	q.head = &SegQueueNode{}
	q.tail = q.head
	q.snToNodeMap = make(map[uint32]*SegQueueNode)
	return q
}

func (q *SegQueue) Back() *SegQueueNode {
	return q.tail
}

func (q *SegQueue) Push(seg *KCPSEG) {
	newNode := &SegQueueNode{Seg: seg}
	newNode.Prev = q.tail
	q.tail.Next = newNode
	q.tail = newNode
	q.len ++
}

func (q *SegQueue) Pop() {
	if q.len == 0 {
		return
	}
	q.tail = q.tail.Prev
	q.len --
}

func (q *SegQueue) Size() int{
	return q.len
}

func (q *SegQueue) ParseAck(sn uint32) {
	node, ok := q.snToNodeMap[sn]
	if !ok {
		return
	}
	node.Prev.Next = node.Next
	q.len --
}

func (q *SegQueue) ParseUna(sn uint32) {
	if q.len == 0 {
		return
	}
	p := q.head.Next
	for p != nil {
		if p.Seg.Una <= sn {
			p.Prev.Next = p.Next
			q.len --
		} else {
			break
		}
	}
}
