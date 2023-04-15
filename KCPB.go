package mykcp

const (
	IKCP_OVERHEAD = 24
	IKCP_PROBE_INIT = 7000
	IKCP_PROBE_LIMIT = 120000
	IKCP_SEND_WIND_FLAG = 1
	IKCP_SEND_WASK_FLAG = 2
	IKCP_THRESH_MIN = 2
)

const (
	IKCP_CMD_PUSH = iota
	IKCP_CMD_ACK
	IKCP_CMD_WASK
	IKCP_CMD_WINS
)

type AckNode struct {
	Sn uint32
	Ts uint32
}
type KCPB struct {
	una          uint32
	mss          uint32
	current      uint32
	updated      uint32
	sndUna       uint32
	tsFlush      uint32
	interval     uint32
	conv         uint32
	stream       int
	sndQueue     *SegQueue
	rcvQueue     *SegQueue
	sndBuf		 *SegQueue
	rcvBuf 	     *SegQueue
	WindRcvLen   uint32
	buffer       []byte
	OverHeadSize uint32
	rmtWnd       uint16
	sndNext      uint32
	rcvNext      uint32
	rcvWind      uint16
	sndWind      uint16
	mtu			 uint32
	ackList      []AckNode
	sendBitFlag uint32
	probeWait	uint32
	tsWait		uint32
	noCwnd	    bool
	cwnd		uint16
	RxRot		uint32
	fastAckThreshould      uint32
	fastXmitLimit	uint32
	state 			int
	deadLinkXmitLimit	uint32
	sshthresh uint32
	incr 	  uint16
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
		kcp.tsFlush = kcp.current
	}

	slap := int(kcp.current) - int(kcp.tsFlush)
	if slap >= 10000 || slap < -10000 {
		kcp.tsFlush = kcp.current
		slap = 0
	}

	if slap >= 0 {
		kcp.tsFlush += kcp.interval
		if kcp.current >= kcp.tsFlush {
			kcp.tsFlush = kcp.current + kcp.interval
		}
		kcp.Flush()
	}
}

func (kcp *KCPB) ShrinkBuf() {
	if kcp.rcvBuf.Size() == 0 {
		kcp.sndUna = kcp.sndNext
		return
	}
	kcp.sndUna = kcp.rcvBuf.Front().Seg.Una
}


func (kcp *KCPB) getUnusedWindSize() uint16 {
	if kcp.rcvQueue.Size() > int(kcp.rcvWind) {
		return 0
	}
	return kcp.rcvWind - uint16(kcp.rcvBuf.Size())
}

func (kcp *KCPB) output(data []byte) {
	// TODO: code it later
}

func (kcp *KCPB) Flush() {

	lost := false
	change := 0
	if kcp.updated == 0 {
		return
	}
	
	current := kcp.current
	buffer := make([]byte, kcp.mtu)
	var seg KCPSEG
	seg.Conv = kcp.conv
	seg.Cmd = IKCP_CMD_ACK
	seg.Wnd = kcp.getUnusedWindSize()

	flushBufferFun := func (buffer []byte) {
		if len(buffer) + IKCP_OVERHEAD > int(kcp.mtu) {
			kcp.output(buffer)
			buffer = make([]byte, kcp.mtu)
		}
	}	
	for i := 0; i < len(kcp.ackList); i++ {
		flushBufferFun(buffer)
		seg.Sn = kcp.ackList[i].Sn
		seg.Ts = kcp.ackList[i].Ts
		buffer = append(buffer, seg.Encode()...)
	}
	kcp.ackList = nil
	
	if kcp.rmtWnd == 0 {
		if kcp.probeWait == 0 {
			kcp.probeWait = IKCP_PROBE_INIT
			kcp.tsWait = kcp.current + kcp.probeWait
			
		} else if kcp.tsWait > kcp.current {
			kcp.probeWait += kcp.probeWait / 2
			if kcp.probeWait > IKCP_PROBE_LIMIT {
				kcp.probeWait = IKCP_PROBE_LIMIT
			}
			kcp.tsWait = kcp.current + kcp.probeWait
			kcp.sendBitFlag |= IKCP_SEND_WASK_FLAG
		}
		
	} else {
		kcp.probeWait = 0
		kcp.tsWait = 0
	}

	if kcp.sendBitFlag & IKCP_SEND_WASK_FLAG > 0{
		seg.Cmd = IKCP_CMD_WASK
		flushBufferFun(buffer)
		buffer = append(buffer, seg.Encode()...)
	}

	if kcp.sendBitFlag & IKCP_SEND_WIND_FLAG > 0 {
		seg.Cmd = IKCP_CMD_WINS
		flushBufferFun(buffer)
		buffer = append(buffer, seg.Encode()...)
	}

	kcp.sendBitFlag = 0
	cwnd := kcp.sndWind
	if cwnd > kcp.rmtWnd {
		cwnd = kcp.rmtWnd
	}
	if kcp.noCwnd { // TODO: check it
		cwnd = kcp.cwnd
	}
	for kcp.sndNext <= kcp.sndUna + uint32(cwnd) {
		if kcp.sndQueue.Size() == 0 {
			break
		}
		seg := kcp.sndQueue.Front().Seg
		kcp.sndQueue.PopFront()
		seg.Conv = kcp.conv
		seg.Cmd = IKCP_CMD_PUSH
		seg.Ts = current
		seg.Sn = kcp.sndNext
		kcp.sndNext++
		seg.Una = kcp.rcvNext
		seg.Resendts = current
		seg.Rto = kcp.RxRot
		seg.Fastack = 0
		seg.Xmit = 0
		kcp.rcvQueue.Push(seg)
	}

	// TODO: delete rotmin
   for p := kcp.sndBuf.Front(); p != nil; p = p.Next {
		seg := p.Seg
		needSend := 0
		if seg.Xmit == 0 {
			needSend = 1
			seg.Xmit = 1
			seg.Rto = kcp.RxRot
			seg.Resendts = current + seg.Rto
		} else if current > seg.Resendts {
			needSend = 1
			seg.Xmit++
			seg.Rto += kcp.RxRot / 2 // change origin kcp caculate way to add 0.5 rxrto directly
			seg.Resendts = current + seg.Rto
			lost = true
		} else if seg.Fastack >= kcp.fastAckThreshould {
			if(seg.Xmit <= kcp.fastXmitLimit || kcp.fastXmitLimit == 0) {
				needSend = 1
				seg.Xmit ++
				seg.Fastack = 0
				seg.Resendts = current + seg.Rto
				change ++
			}
		}
		if needSend > 0 {
			seg.Ts = current
			seg.Wnd = kcp.getUnusedWindSize()
			seg.Una = kcp.rcvNext
			flushBufferFun(buffer)
			buffer = append(buffer, seg.Encode()...)
			if seg.Len > 0 {
				buffer = append(buffer, seg.Data...)
			}
			if seg.Xmit >= kcp.deadLinkXmitLimit {
				kcp.state = -1

			}
		}
		
   }
   if len(buffer) > 0 {
		kcp.output(buffer)
		buffer = nil
	}

	if change > 0 { // 快速重传
		inflight := kcp.sndNext - kcp.sndUna
		kcp.sshthresh = inflight / 2
		if kcp.sshthresh < IKCP_THRESH_MIN {
			kcp.sshthresh = IKCP_THRESH_MIN
		}
		kcp.cwnd = uint16(kcp.sshthresh) + uint16(kcp.fastXmitLimit)
		kcp.incr = kcp.cwnd * uint16(kcp.mss)

	}

	if lost {
		kcp.sshthresh = uint32(cwnd) / 2
		if kcp.sshthresh < IKCP_THRESH_MIN {
			kcp.sshthresh = IKCP_THRESH_MIN
		}
		kcp.cwnd = 1
		kcp.incr = uint16(kcp.mss)
	}

	if kcp.cwnd < 1 {
		kcp.cwnd = 1
		kcp.incr = uint16(kcp.mss)
	}
	
}

func (kcp *KCPB) updateRcvQueue() {
	for kcp.rcvBuf.Size() != 0 {
		if kcp.rcvBuf.Back().Seg.Sn == kcp.rcvNext && kcp.rcvQueue.Size() <= int(kcp.rcvWind) {
			seg := kcp.rcvBuf.Back()
			kcp.rcvQueue.Push(seg.Seg)
			kcp.rcvBuf.PopBack()
		} else {
			break
		}
	}
}

func (kcp *KCPB) Input(data []byte) int {
	prev_una := kcp.sndUna
	var maxAck uint32
	//var lastestTs uint32
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
		if seg.Cmd != IKCP_CMD_ACK && seg.Cmd != IKCP_CMD_WINS && seg.Cmd != IKCP_CMD_WASK && seg.Cmd != IKCP_CMD_PUSH {
			return -3
		}
		kcp.rmtWnd = seg.Wnd
		kcp.sndBuf.ParseUna(seg.Una)
		kcp.ShrinkBuf() // TODO: check it if can be put back

		if seg.Cmd == IKCP_CMD_ACK { // TODO: finish rto
			if flag == 0 {
				flag = 1
				maxAck = seg.Una
			} else if maxAck < seg.Una {
				maxAck = seg.Una
			}
			kcp.sndBuf.ParseAck(seg.Una)
			kcp.ShrinkBuf()
		} else if seg.Cmd == IKCP_CMD_PUSH {
			if kcp.rcvNext+uint32(kcp.rcvWind) > seg.Una {
				kcp.ackList = append(kcp.ackList, AckNode{seg.Sn, seg.Ts})
				seg.Data = make([]byte, seg.Len)
				copy(seg.Data, data)
				kcp.rcvBuf.PushSegment(&seg)
				kcp.updateRcvQueue()
			}
		} else if seg.Cmd == IKCP_CMD_WASK {
			kcp.sendBitFlag |= IKCP_SEND_WIND_FLAG
		} else if seg.Cmd == IKCP_CMD_WINS {
			// have been processed before
		} else {
			return -3
		}
		data = data[seg.Len:]

	}
	
	if flag != 0 {
		kcp.sndBuf.ParseFastAck(maxAck)
	}
	//todo: finish cwnd
	return 0
}
