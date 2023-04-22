package main

import "log"

const (
	IKCP_OVERHEAD       = 24
	IKCP_PROBE_INIT     = 7000
	IKCP_PROBE_LIMIT    = 120000
	IKCP_SEND_WIND_FLAG = 1
	IKCP_SEND_WASK_FLAG = 2
	IKCP_THRESH_MIN     = 2
	IKCP_RTO_MAX        = 60000
	IKCP_WND_SND        = 32
	IKCP_WND_RCV        = 128
	IKCP_MTE_DEF        = 1400
	IKCP_RTO_DEF        = 200
	IKCP_RTO_MIN        = 100
	IKCP_INTERVAL       = 100
	IKCP_SSHTHRESH_INIT = 2
	IKCP_FASTACK_LIMIT  = 5
	IKCP_DEAD_LINK_MAX  = 5
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
	mss               uint32
	current           uint32
	updated           uint32
	sndUna            uint32
	tsFlush           uint32
	interval          uint32
	conv              uint32
	stream            int
	sndQueue          *SegQueue
	rcvQueue          *SegQueue
	sndBuf            *SegQueue
	rcvBuf            *SegQueue
	WindRcvLen        uint32
	OverHeadSize      uint32
	rmtWnd            uint16
	sndNext           uint32
	rcvNext           uint32
	rcvWind           uint16
	sndWind           uint16
	mtu               uint32
	ackList           []AckNode
	sendBitFlag       uint32
	probeWait         uint32
	tsWait            uint32
	noCwnd            bool
	cwnd              uint16
	RxRto             uint32
	RxSrtt            uint32
	RxRttval          uint32
	fastAckThreshould uint32
	fastXmitLimit     uint32
	state             int
	deadLinkXmitLimit uint32
	sshthresh         uint32
	incr              uint16
	outPutFun         func(data []byte)
}

func NewKcpB(conv uint32) *KCPB {
	kcpb := &KCPB{}
	kcpb.conv = conv
	kcpb.sndWind = IKCP_WND_SND
	kcpb.rcvWind = IKCP_WND_RCV
	kcpb.rmtWnd = IKCP_WND_RCV
	kcpb.mtu = IKCP_MTE_DEF
	kcpb.mss = kcpb.mtu - IKCP_OVERHEAD
	kcpb.rcvQueue = NewSegQueue()
	kcpb.sndQueue = NewSegQueue()
	kcpb.rcvBuf = NewSegQueue()
	kcpb.sndBuf = NewSegQueue()
	kcpb.ackList = make([]AckNode, 0)
	kcpb.RxRto = IKCP_RTO_DEF
	kcpb.interval = IKCP_INTERVAL
	kcpb.tsFlush = IKCP_INTERVAL
	kcpb.sshthresh = IKCP_SSHTHRESH_INIT
	kcpb.fastAckThreshould = IKCP_FASTACK_LIMIT
	kcpb.deadLinkXmitLimit = IKCP_DEAD_LINK_MAX
	return kcpb
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
	}
	size = len(buffer)
	count := 0
	if size <= int(kcp.mss) {
		count = 1
	} else {
		count = (size + int(kcp.mss) - 1) / int(kcp.mss)
	}
	if count >= int(kcp.rcvWind) {
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
		newSeg.Sn = kcp.sndNext
		kcp.sndNext++

		if kcp.stream == 0 {
			newSeg.Frg = 0
		}
		kcp.sndQueue.Push(&newSeg)
		buffer = buffer[segSize:]
		size = len(buffer)
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
	kcp.outPutFun(data)
}

func (kcp *KCPB) Flush() {

	lost := false
	change := 0
	if kcp.updated == 0 {
		return
	}

	current := kcp.current
	buffer := make([]byte, 0)
	var seg KCPSEG
	seg.Conv = kcp.conv
	seg.Cmd = IKCP_CMD_ACK
	seg.Wnd = kcp.getUnusedWindSize()
	seg.Una = kcp.rcvNext

	flushBufferFun := func(buffer []byte) []byte {
		if len(buffer)+IKCP_OVERHEAD > int(kcp.mtu) {
			kcp.output(buffer)
			buffer = make([]byte, 0)

		}
		return buffer
	}

	for i := 0; i < len(kcp.ackList); i++ {
		buffer = flushBufferFun(buffer)
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

	if kcp.sendBitFlag&IKCP_SEND_WASK_FLAG > 0 {
		seg.Cmd = IKCP_CMD_WASK
		buffer = flushBufferFun(buffer)
		buffer = append(buffer, seg.Encode()...)
	}

	if kcp.sendBitFlag&IKCP_SEND_WIND_FLAG > 0 {
		seg.Cmd = IKCP_CMD_WINS
		buffer = flushBufferFun(buffer)
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
	for kcp.sndNext <= kcp.sndUna+uint32(cwnd) {
		if kcp.sndQueue.Size() == 0 {
			break
		}
		seg := kcp.sndQueue.Front().Seg
		kcp.sndQueue.PopFront()
		seg.Conv = kcp.conv
		seg.Cmd = IKCP_CMD_PUSH
		seg.Ts = current
		seg.Una = kcp.rcvNext
		seg.Resendts = current
		seg.Rto = kcp.RxRto
		seg.Fastack = 0
		seg.Xmit = 0
		kcp.sndBuf.Push(seg)
	}

	// TODO: delete rotmin
	for p := kcp.sndBuf.Front(); p != nil; p = p.Next {
		seg := p.Seg
		needSend := 0
		if seg.Xmit == 0 {
			needSend = 1
			seg.Xmit = 1
			seg.Rto = kcp.RxRto
			seg.Resendts = current + seg.Rto
		} else if current > seg.Resendts {
			needSend = 1
			seg.Xmit++
			seg.Rto += kcp.RxRto / 2 // change origin kcp caculate way to add 0.5 rxrto directly
			seg.Resendts = current + seg.Rto
			lost = true
		} else if seg.Fastack >= kcp.fastAckThreshould {
			if seg.Xmit <= kcp.fastXmitLimit || kcp.fastXmitLimit == 0 {
				needSend = 1
				seg.Xmit++
				seg.Fastack = 0
				seg.Resendts = current + seg.Rto
				change++
			}
		}
		if needSend > 0 {
			seg.Ts = current
			seg.Wnd = kcp.getUnusedWindSize()
			seg.Una = kcp.rcvNext
			seg.Len = uint32(len(seg.Data))
			buffer = flushBufferFun(buffer)
			buffer = append(buffer, seg.Encode()...)
			log.Printf("send data sn is %v", seg.Sn)
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
		if kcp.rcvBuf.Front().Seg.Sn == kcp.rcvNext && kcp.rcvQueue.Size() < int(kcp.rcvWind) {
			seg := kcp.rcvBuf.Front()
			kcp.rcvQueue.Push(seg.Seg)
			kcp.rcvBuf.PopFront()
			kcp.rcvNext++
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
		if len(data) < IKCP_OVERHEAD {
			break
		}

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

		if seg.Cmd == IKCP_CMD_ACK {
			if kcp.current > seg.Ts {
				kcp.updateRto(kcp.current - seg.Ts)
			}
			log.Println("receive ACk, sn is %v", seg.Sn)
			if flag == 0 {
				flag = 1
				maxAck = seg.Una
			} else if maxAck < seg.Una {
				maxAck = seg.Una
			}
			kcp.sndBuf.ParseAck(seg.Sn)
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
	if kcp.sndUna > prev_una {
		if kcp.cwnd < kcp.rmtWnd {
			mss := kcp.mss
			if kcp.cwnd < uint16(kcp.sshthresh) {
				kcp.cwnd++
				kcp.incr += uint16(mss)

			} else {
				if kcp.incr < uint16(mss) {
					kcp.incr = uint16(mss)
				}
				kcp.incr += (uint16(kcp.mss)*uint16(kcp.mss))/kcp.incr + (uint16(mss) / 16)
				if (kcp.cwnd+1)*uint16(kcp.mss) <= kcp.incr {
					kcp.cwnd++
				}
			}
			if kcp.cwnd > kcp.rmtWnd {
				kcp.cwnd = kcp.rmtWnd
				kcp.incr = kcp.rmtWnd * uint16(mss)
			}
		}
	}
	return 0
}

func (kcp *KCPB) SetOutPut(outPutFun func(data []byte)) {
	kcp.outPutFun = outPutFun
}

func (kcp *KCPB) updateRto(rtt uint32) {
	var rto uint32
	if kcp.RxSrtt == 0 {
		kcp.RxSrtt = rtt
		kcp.RxRttval = rtt / 2
	} else {
		delta := rtt - kcp.RxSrtt
		if rtt < kcp.RxSrtt {
			delta = kcp.RxSrtt - rtt
		}
		kcp.RxRttval = (3*kcp.RxRttval + delta) / 4
		kcp.RxSrtt = (7*kcp.RxSrtt + rtt) / 8
		if kcp.RxSrtt < 1 {
			kcp.RxSrtt = 1
		}
		maxBias := kcp.interval
		if maxBias < 4*kcp.RxRttval {
			maxBias = 4 * kcp.RxRttval
		}
		rto = kcp.RxSrtt + maxBias
		kcp.RxRto = rto
		if kcp.RxRto > IKCP_RTO_MAX {
			kcp.RxRto = IKCP_RTO_MAX
		}
		if kcp.RxRto < IKCP_RTO_MIN {
			kcp.RxRto = IKCP_RTO_MIN
		}

	}
}

func (kcp *KCPB) getNextRecvPacketSize() int {
	if kcp.rcvQueue.Size() == 0 || kcp.rcvQueue.Size() < int(kcp.rcvQueue.Front().Seg.Frg) {
		return -1
	}
	seg := kcp.rcvQueue.Front().Seg
	if seg.Frg == 0 {
		return int(seg.Len)
	}

	var len uint32
	for p := kcp.rcvQueue.Front(); p != nil; p = p.Next {
		len += p.Seg.Len
		if p.Seg.Frg == 0 {
			break
		}
	}
	return 0
}

func (kcp *KCPB) Recv(buffer []byte) int {
	if kcp.rcvQueue.Size() == 0 {
		return -1
	}
	nextPacketSize := kcp.getNextRecvPacketSize()
	if nextPacketSize < 0 {
		return -1
	}
	recoverFlag := false
	if kcp.rcvQueue.Size() >= int(kcp.rcvWind) {
		recoverFlag = true
	}

	for kcp.rcvQueue.Size() != 0 {
		p := kcp.rcvQueue.Front()
		seg := p.Seg

		copy(buffer, seg.Data)
		buffer = buffer[len(seg.Data):]
		kcp.rcvQueue.PopFront()
		if seg.Frg == 0 {
			break
		}
	}

	for kcp.rcvBuf.Size() > 0 {
		seg := kcp.rcvBuf.Front().Seg
		if seg.Sn == kcp.rcvNext && kcp.rcvQueue.Size() < int(kcp.rcvWind) {
			kcp.rcvBuf.PopFront()
			kcp.rcvQueue.PushSegment(seg)
			kcp.rcvNext++
		} else {
			break
		}
	}

	if kcp.rcvQueue.Size() < int(kcp.rcvWind) && recoverFlag {
		kcp.sendBitFlag |= IKCP_SEND_WIND_FLAG
	}

	return 0
}
