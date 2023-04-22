package main

type KCPSEG struct {
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

func newKcpSeg(size int) KCPSEG {
	kcpSeg := KCPSEG{}
	kcpSeg.Data = make([]byte, size)
	return kcpSeg
}

func (seg *KCPSEG) Encode() []byte {

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
	Seg  *KCPSEG
}

type SegQueue struct {
	head        *SegQueueNode
	tail        *SegQueueNode
	len         int
	snToNodeMap map[uint32]*SegQueueNode
}

func NewSegQueue() *SegQueue {
	q := &SegQueue{}
	q.head = &SegQueueNode{}
	q.tail = q.head
	q.snToNodeMap = make(map[uint32]*SegQueueNode)
	q.len = 0
	return q
}

func (q *SegQueue) Front() *SegQueueNode {
	return q.head.Next
}
func (q *SegQueue) Back() *SegQueueNode {
	return q.tail
}

func (q *SegQueue) Push(seg *KCPSEG) {
	if _, ok := q.snToNodeMap[seg.Sn]; ok {
		return
	}

	newNode := &SegQueueNode{Seg: seg}
	newNode.Prev = q.tail
	q.tail.Next = newNode
	q.tail = newNode
	q.snToNodeMap[seg.Sn] = newNode
	q.len++
}

func (q *SegQueue) PopFront() {
	if q.len == 0 {
		return
	}
	delete(q.snToNodeMap, q.head.Next.Seg.Una)
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
	delete(q.snToNodeMap, q.tail.Seg.Una)
	q.tail = q.tail.Prev
	q.tail.Next = nil
	q.len--
	if q.len == 0 {
		q.tail = q.head
		q.head.Next = nil
	}
}

func (q *SegQueue) deleteSeg(sn uint32) {
	node, ok := q.snToNodeMap[sn]
	if !ok {
		return
	}
	node.Prev.Next = node.Next
	if node.Next != nil {
		node.Next.Prev = node.Prev
	}
	delete(q.snToNodeMap, sn)
	q.len--
	if q.len == 0 {
		q.tail = q.head
		q.head.Next = nil
	}
}

func (q *SegQueue) Size() int {
	return q.len
}

func (q *SegQueue) ParseAck(sn uint32) {
	q.deleteSeg(sn)
}

func (q *SegQueue) ParseUna(sn uint32) {
	if q.len == 0 {
		return
	}
	p := q.head.Next
	for p != nil {
		nextNode := p.Next
		if p.Seg.Una <= sn {
			q.deleteSeg(p.Seg.Una)
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

func (q *SegQueue) PushSegment(seg *KCPSEG) {
	_, ok := q.snToNodeMap[seg.Sn]
	if ok {
		return
	}
	//p := q.head.Next
	//for p.Next != nil && p.Next.Seg.Sn < seg.Sn {
	//	p = p.Next
	//}
	//segNode := &SegQueueNode{Seg: seg}
	//segNode.Next = p.Next
	//if p.Next != nil {
	//	p.Next.Prev = segNode
	//}
	//segNode.Prev = p
	//p.Next = segNode
	q.Push(seg)
}
