package Rcp

type RcpSeg struct {
	Conv     uint32
	Cmd      uint8
	Frg      uint8
	Wnd      uint16
	Ts       uint32
	Sn       uint32
	Una      uint32
	Len      uint32
	Resendts uint32
	Rto      uint32
	Fastack  uint32
	Xmit     uint32
	Data     []byte
}

func newRcpSeg(size int) RcpSeg {
	kcpSeg := RcpSeg{}
	kcpSeg.Data = make([]byte, size)
	return kcpSeg
}

func (seg *RcpSeg) Encode() []byte {

	data := make([]byte, 24)
	returnData := data
	data = ikcp_encode32u(data, seg.Conv)
	data = ikcp_encode8u(data, seg.Cmd)
	data = ikcp_encode8u(data, seg.Frg)
	data = ikcp_encode16u(data, seg.Wnd)
	data = ikcp_encode32u(data, seg.Ts)
	data = ikcp_encode32u(data, seg.Sn)
	data = ikcp_encode32u(data, seg.Una)
	data = ikcp_encode32u(data, seg.Len)
	return returnData
}

type SegQueueNode struct {
	Next *SegQueueNode
	Prev *SegQueueNode
	Seg  *RcpSeg
}

type SegQueue struct {
	head *SegQueueNode
	tail *SegQueueNode
	len  int
	//snToNodeMap map[uint32]*SegQueueNode
}

func NewSegQueue() *SegQueue {
	q := &SegQueue{}
	q.head = &SegQueueNode{}
	q.tail = q.head
	//q.snToNodeMap = make(map[uint32]*SegQueueNode)
	q.len = 0
	return q
}

func (q *SegQueue) Front() *SegQueueNode {
	return q.head.Next
}
func (q *SegQueue) Back() *SegQueueNode {
	return q.tail
}

func (q *SegQueue) Push(seg *RcpSeg) {
	q.PushSegment(seg)
}

func (q *SegQueue) PopFront() {
	if q.len == 0 {
		return
	}
	q.head.Next = q.head.Next.Next
	if q.head.Next != nil {
		q.head.Next.Prev = q.head
	}

	q.len--
	if q.len == 0 {
		q.tail = q.head
		q.head.Next = nil
	}

}
func (q *SegQueue) PopBack() {
	if q.len == 0 {
		return
	}
	q.tail = q.tail.Prev
	q.tail.Next = nil
	q.len--
	if q.len == 0 {
		q.tail = q.head
		q.head.Next = nil
	}
}

func (q *SegQueue) deleteSeg(node *SegQueueNode) {

	if node == q.tail {
		q.tail = node.Prev
	}
	node.Prev.Next = node.Next
	if node.Next != nil {
		node.Next.Prev = node.Prev
	}
	q.len--
	if q.len == 0 {
		q.tail = q.head
		q.head.Next = nil
	}
}

func (q *SegQueue) ParseAck(sn uint32) {
	for p := q.Front(); p != nil; p = p.Next {
		if p.Seg.Sn > sn {
			return
		}
		if p.Seg.Sn == sn {
			q.deleteSeg(p)
			return
		}
	}
}

func (q *SegQueue) Size() int {
	return q.len
}

func (q *SegQueue) ParseUna(sn uint32) {
	if q.len == 0 {
		return
	}
	p := q.head.Next
	for p != nil {
		nextNode := p.Next
		if p.Seg.Sn < sn {
			q.deleteSeg(p)
		} else {
			break
		}
		p = nextNode
	}
}

func (q *SegQueue) ParseFastAck(sn uint32) {
	if q.len == 0 {
		return
	}
	p := q.head.Next
	for p != nil {
		if p.Seg.Sn < sn {
			p.Seg.Fastack++
		} else {
			break
		}
		p = p.Next
	}
}

func (q *SegQueue) PushSegment(seg *RcpSeg) {
	segNode := &SegQueueNode{Seg: seg}
	if q.Size() == 0 {
		q.head.Next = segNode
		segNode.Prev = q.head
		q.tail = segNode
	} else {
		p := q.head
		for p.Next != nil && p.Next.Seg.Sn < seg.Sn {
			p = p.Next
		}
		if p.Next != nil && p.Next.Seg.Sn == seg.Sn {
			return
		}
		segNode.Next = p.Next
		if p.Next != nil {
			p.Next.Prev = segNode
		} else {
			q.tail = segNode
		}
		segNode.Prev = p
		p.Next = segNode
	}
	q.len++
	//q.Push(seg)
}
