package mykcp

const IKCP_OVERHEAD = 24

const (
	IKCP_CMD_PUSH = iota
	IKCP_CMD_ACK
	IKCP_CMD_WASK
	IKCP_CMD_WINS
)

type KCPB struct {
	una          uint32
	mss          uint32
	current      uint32
	updated      uint32
	sndUna       uint32
	ts_flush     uint32
	interval     uint32
	conv         uint32
	stream       int
	sndQueue     *SegQueue
	rcv_queue    *SegQueue
	WindRcvLen   uint32
	buffer       []byte
	OverHeadSize uint
}

func (kcp *KCPB) Send(buffer []byte) int {
	size := len(buffer)
	if kcp.stream != 0 {
		queueSize := kcp.sndQueue.Size()
		if queueSize > 0 {
			old := kcp.sndQueue.Back().Seg
			if len(old.Data) < int(kcp.mss) {
				capacity := int(kcp.mss) - len(old.Data)
				extend := capacity
				if extend > size {
					extend = size
				}
				oldLen := len(old.Data)
				old.Data = old.Data[:oldLen+extend]
				copy(old.Data[oldLen:], buffer)
				buffer = buffer[extend:]
			}
		}
		if len(buffer) <= 0 {
			return 0
		}
		size = len(buffer)
		count := 0
		if size <= int(kcp.mss) {
			count = 1
		} else {
			count = (size + int(kcp.mss) - 1) / int(kcp.mss)
		}
		if count >= int(kcp.WindRcvLen) {
			return -2
		}
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			if size == 0 {
				break
			}
			segSize := kcp.mss
			if int(segSize) > size {
				segSize = uint32(size)
			}
			newSeg := newKcpSeg(int(segSize))
			copy(newSeg.Data, buffer)
			newSeg.Frg = uint8(count - i - 1)
			if kcp.stream == 0 {
				newSeg.Frg = 0
			}
			kcp.sndQueue.Push(&newSeg)
			buffer = buffer[segSize:]
			size = len(buffer)
		}

	}
	return 0
}

func (kcp *KCPB) Update(current uint32) {
	kcp.current = current
	if kcp.updated == 0 {
		kcp.updated = 1
		kcp.ts_flush = kcp.current
	}

	slap := int(kcp.current) - int(kcp.ts_flush)
	if slap >= 10000 || slap < -10000 {
		kcp.ts_flush = kcp.current
		slap = 0
	}

	if slap >= 0 {
		kcp.ts_flush += kcp.interval
		if kcp.current >= kcp.ts_flush {
			kcp.ts_flush = kcp.current + kcp.interval
		}
		kcp.Flush()
	}
}

func (kcp *KCPB) Flush() {
	if kcp.updated == 0 {
		return
	}

	current := kcp.current
	buffer := kcp.buffer
	var seg KCPSEG
	seg.Conv = kcp.conv
	// seg.Cmd =
}

func (kcp *KCPB) Input(data []byte) int {
	prev_una := kcp.sndUna
	var maxAck uint32
	var lastestTs uint32
	flag := 0
	for {
		var seg KCPSEG
		data = ikcp_decode32u(data, &seg.Conv)
		if seg.Conv != kcp.conv {
			return -1
		}
		data = ikcp_decode8u(data, &seg.Cmd)
		data = ikcp_decode8u(data, &seg.Frg)
		data = ikcp_decode16u(data, &seg.Wnd)
		data = ikcp_decode32u(data, &seg.Ts)
		data = ikcp_decode32u(data, &seg.Sn)
		data = ikcp_decode32u(data, &seg.Una)
		data = ikcp_decode32u(data, &seg.Len)

		if len(data) < int(seg.Len) {
			return -2
		}

	}

}

// func (kcp *KCPB) ikcp_input(data []byte) int {
// 	size := len(data)
// 	if(size == 0) {
// 		return -1
// 	}
// 	while(1) {

// 	}
// }
