package Rcp

import "time"

const (
	IKCP_OVERHEAD       = 24
	IKCP_PROBE_INIT     = 7000
	IKCP_PROBE_LIMIT    = 120000
	IKCP_SEND_WIND_FLAG = 1
	IKCP_SEND_WASK_FLAG = 2
	IKCP_THRESH_MIN     = 2
	IKCP_RTO_MAX        = 60000
	IKCP_WND_SND        = 700
	IKCP_WND_RCV        = 700
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

type Rcpb struct {
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
	buffer            RcpSendBuffer
	debugName         string // For Debug

	//For Test
	SendMask map[uint32]struct{}

	//For Speed Test
	lastSpeedTime uint32
	SpeedTimeSum  uint32
	sendByteSum   uint32
}

func NewKcpB(conv uint32, localAddr string) *Rcpb {
	rcpb := &Rcpb{}
	rcpb.conv = conv
	rcpb.sndWind = IKCP_WND_SND
	rcpb.rcvWind = IKCP_WND_RCV
	rcpb.rmtWnd = IKCP_WND_RCV
	rcpb.mtu = IKCP_MTE_DEF
	rcpb.mss = rcpb.mtu - IKCP_OVERHEAD
	rcpb.rcvQueue = NewSegQueue()
	rcpb.sndQueue = NewSegQueue()
	rcpb.rcvBuf = NewSegQueue()
	rcpb.sndBuf = NewSegQueue()
	rcpb.ackList = make([]AckNode, 0)
	rcpb.RxRto = IKCP_RTO_DEF
	rcpb.interval = IKCP_INTERVAL
	rcpb.tsFlush = IKCP_INTERVAL
	rcpb.sshthresh = IKCP_SSHTHRESH_INIT
	rcpb.fastAckThreshould = IKCP_FASTACK_LIMIT
	rcpb.deadLinkXmitLimit = IKCP_DEAD_LINK_MAX
	rcpb.debugName = "Block " + localAddr
	rcpb.SendMask = make(map[uint32]struct{})
	rcpb.cwnd = 1
	rcpb.buffer.Init(5000, int(rcpb.mtu-100))
	return rcpb
}

func (rcp *Rcpb) Close() {
	rcp.state = -1
	rcp.Flush()
}

func (rcp *Rcpb) Send(buffer []byte) int {
	size := len(buffer)
	if rcp.stream != 0 {
		queueSize := rcp.sndQueue.Size()
		if queueSize > 0 {
			old := rcp.sndQueue.Back().Seg
			if len(old.Data) < int(rcp.mss) {
				capacity := int(rcp.mss) - len(old.Data)
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
	if size <= int(rcp.mss) {
		count = 1
	} else {
		count = (size + int(rcp.mss) - 1) / int(rcp.mss)
	}
	if count >= int(rcp.sndWind) {
		return -2
	}
	if count == 0 {
		count = 1
	}
	for i := 0; i < count; i++ {
		if size == 0 {
			break
		}
		segSize := rcp.mss
		if int(segSize) > size {
			segSize = uint32(size)
		}
		newSeg := newRcpSeg(int(segSize))
		copy(newSeg.Data, buffer)
		newSeg.Frg = uint8(count - i - 1)
		newSeg.Sn = rcp.sndNext
		rcp.sndNext++

		if rcp.stream == 0 {
			newSeg.Frg = 0
		}
		rcp.sndQueue.Push(&newSeg)
		buffer = buffer[segSize:]
		size = len(buffer)
	}

	return 0
}

func (rcp *Rcpb) Update(current uint32) {
	rcp.current = current
	if rcp.updated == 0 {
		rcp.updated = 1
		rcp.tsFlush = rcp.current
	}

	slap := int(rcp.current) - int(rcp.tsFlush)
	if slap >= 10000 || slap < -10000 {
		rcp.tsFlush = rcp.current
		slap = 0
	}

	if slap >= 0 {
		rcp.tsFlush += rcp.interval
		if rcp.current >= rcp.tsFlush {
			rcp.tsFlush = rcp.current + rcp.interval
		}
		rcp.Flush()
	}
}

func (rcp *Rcpb) ShrinkBuf() {
	if rcp.sndBuf.Size() == 0 {
		if rcp.sndQueue.Size() == 0 {
			rcp.sndUna = rcp.sndNext
			return
		}
		rcp.sndUna = rcp.sndQueue.Front().Seg.Sn
		return
	}
	rcp.sndUna = rcp.sndBuf.Front().Seg.Sn
}

func (rcp *Rcpb) getUnusedWindSize() uint16 {
	if rcp.rcvQueue.Size()+rcp.rcvBuf.Size() > int(rcp.rcvWind) {
		return 0
	}
	return rcp.rcvWind - (uint16(rcp.rcvQueue.Size()) + uint16(rcp.rcvBuf.Size()))
}

func (rcp *Rcpb) output(data []byte) {
	rcp.outPutFun(data)
}

func (rcp *Rcpb) ClearUpSendSpeedTime() {
	rcp.sendByteSum = 0
	rcp.SpeedTimeSum = 0
	rcp.lastSpeedTime = 0
}

func (rcp *Rcpb) updateSendSum(sendSize int) {
	timeNow := uint32(time.Now().UnixMilli())
	rcp.sendByteSum += uint32(sendSize)
	if rcp.lastSpeedTime > 0 {
		rcp.SpeedTimeSum += timeNow - rcp.lastSpeedTime
	}
	rcp.lastSpeedTime = timeNow
}
func (rcp *Rcpb) Flush() {
	if rcp.debugName != "Block 127.0.0.1:9666" {
		//println("hi")
	}

	// FIXME: Just For Test
	//lost := false
	change := 0

	current := uint32(time.Now().UnixMilli())
	var seg RcpSeg
	seg.Conv = rcp.conv
	seg.Cmd = IKCP_CMD_ACK
	seg.Wnd = rcp.getUnusedWindSize()
	seg.Una = rcp.rcvNext

	for i := 0; i < len(rcp.ackList); i++ {

		seg.Sn = rcp.ackList[i].Sn
		seg.Ts = rcp.ackList[i].Ts
		RcpDebugPrintf(rcp.debugName, "Send ACK Packet, sn %v", seg.Sn)
		rcp.buffer.SendData(seg.Encode())
	}

	rcp.ackList = nil

	if rcp.rmtWnd == 0 {
		if rcp.probeWait == 0 {
			rcp.probeWait = IKCP_PROBE_INIT
			rcp.tsWait = rcp.current + rcp.probeWait

		} else if rcp.tsWait > rcp.current {
			rcp.probeWait += rcp.probeWait / 2
			if rcp.probeWait > IKCP_PROBE_LIMIT {
				rcp.probeWait = IKCP_PROBE_LIMIT
			}
			rcp.tsWait = rcp.current + rcp.probeWait
			rcp.sendBitFlag |= IKCP_SEND_WASK_FLAG
		}

	} else {
		rcp.probeWait = 0
		rcp.tsWait = 0
	}

	if rcp.sendBitFlag&IKCP_SEND_WASK_FLAG > 0 {
		seg.Cmd = IKCP_CMD_WASK
		RcpDebugPrintf(rcp.debugName, "Send Ask Windows Size packet")
		rcp.buffer.SendData(seg.Encode())
	}

	if rcp.sendBitFlag&IKCP_SEND_WIND_FLAG > 0 {
		seg.Cmd = IKCP_CMD_WINS
		RcpDebugPrintf(rcp.debugName, "Send Tell Windows Size packet, size %v", seg.Wnd)
		rcp.buffer.SendData(seg.Encode())
	}

	rcp.sendBitFlag = 0
	cwnd := rcp.sndWind
	if cwnd > rcp.rmtWnd {
		cwnd = rcp.rmtWnd
	}
	if rcp.noCwnd { // TODO: check it
		cwnd = rcp.cwnd
	}
	for rcp.sndBuf.Size() < int(rcp.cwnd) { // make sure send cwnd size packet at once
		if rcp.sndQueue.Size() == 0 {
			break
		}
		seg := rcp.sndQueue.Front().Seg
		rcp.sndQueue.PopFront()
		seg.Conv = rcp.conv
		seg.Cmd = IKCP_CMD_PUSH
		seg.Ts = current
		seg.Una = rcp.rcvNext
		seg.Resendts = current
		seg.Rto = rcp.RxRto
		seg.Fastack = 0
		seg.Xmit = 0
		rcp.sndBuf.Push(seg)
	}

	// TODO: delete rotmin
	for p := rcp.sndBuf.Front(); p != nil; p = p.Next {
		seg := p.Seg
		needSend := 0
		if seg.Xmit == 0 {
			needSend = 1
			seg.Xmit = 1
			seg.Rto = rcp.RxRto
			seg.Resendts = current + seg.Rto
			rcp.updateSendSum(len(seg.Data))
		} else if current > seg.Resendts {
			needSend = 1
			seg.Xmit++
			seg.Rto += rcp.RxRto / 2 // change origin rcp caculate way to add 0.5 rxrto directly
			seg.Resendts = current + seg.Rto
			//lost = true
		} else if seg.Fastack >= rcp.fastAckThreshould {
			if seg.Xmit <= rcp.fastXmitLimit || rcp.fastXmitLimit == 0 {
				needSend = 1
				seg.Xmit++
				seg.Fastack = 0
				seg.Resendts = current + seg.Rto
				change++
			}
		}
		//if seg.Sn == 1 && rcp.rmtWnd > 0 {
		//	needSend = 0 // TODO: delete it
		//}

		// TODO: For Test
		if _, ok := rcp.SendMask[seg.Sn]; ok {
			needSend = 0
			if seg.Xmit == 1 {
				RcpDebugPrintf(rcp.debugName, "Send Push Packet, sn %v, but blocked", seg.Sn)
			}

		}

		if needSend > 0 {
			RcpDebugPrintf(rcp.debugName, "Send Push Packet, sn %v, data len %v", seg.Sn, len(seg.Data))
			seg.Ts = current
			seg.Wnd = rcp.getUnusedWindSize()
			seg.Una = rcp.rcvNext
			seg.Len = uint32(len(seg.Data))
			if seg.Len > 0 {
				sendData := seg.Encode()
				sendData = append(sendData, seg.Data...)
				rcp.buffer.SendData(sendData)
			} else {
				rcp.buffer.SendData(seg.Encode())
			}
			if seg.Xmit >= rcp.deadLinkXmitLimit {
				rcp.state = -1
			}
		}

	}
	rcp.buffer.Flush()

	if change > 0 { // 快速重传
		inflight := rcp.sndNext - rcp.sndUna
		rcp.sshthresh = inflight / 2
		if rcp.sshthresh < IKCP_THRESH_MIN {
			rcp.sshthresh = IKCP_THRESH_MIN
		}
		rcp.cwnd = uint16(rcp.sshthresh) + uint16(rcp.fastXmitLimit)
		rcp.incr = rcp.cwnd * uint16(rcp.mss)

	}

	//FIXME: just for test
	//if lost {
	//	rcp.sshthresh = uint32(cwnd) / 2
	//	if rcp.sshthresh < IKCP_THRESH_MIN {
	//		rcp.sshthresh = IKCP_THRESH_MIN
	//	}
	//	rcp.cwnd = 1
	//	rcp.incr = uint16(rcp.mss)
	//}

	if rcp.cwnd < 1 {
		rcp.cwnd = 1
		rcp.incr = uint16(rcp.mss)
	}

}

func (rcp *Rcpb) updateRcvQueue() {
	for rcp.rcvBuf.Size() != 0 {
		if rcp.rcvBuf.Front().Seg.Sn <= rcp.rcvNext && rcp.rcvQueue.Size() < int(rcp.rcvWind) {
			seg := rcp.rcvBuf.Front()
			rcp.rcvBuf.PopFront()
			if seg.Seg.Sn != rcp.rcvNext {
				continue
			}
			rcp.rcvQueue.Push(seg.Seg)
			rcp.rcvNext++
		} else {
			break
		}
	}
}

func (rcp *Rcpb) Input(data []byte) int {
	if rcp.debugName != "Block 127.0.0.1:9666" {
		RcpDebugPrintf(rcp.debugName, "Input Get Pakcet Data %v", len(data))
	}
	prevUna := rcp.sndUna
	//pervDat := data
	////println(len(pervDat))
	var maxAck uint32
	//var lastestTs uint32
	flag := 0
	cnt := 0
	for {

		if len(data) < IKCP_OVERHEAD {
			if cnt == 0 {
				RcpDebugPrintf(rcp.debugName, "Input Error -1, Data len %v", len(data))
			}
			break
		}

		var seg RcpSeg
		data = ikcp_decode32u(data, &seg.Conv)
		if seg.Conv != rcp.conv {
			RcpDebugPrintf(rcp.debugName, "Input Error -4, Conv %v, want Conv %v", seg.Conv, rcp.conv)
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
			RcpDebugPrintf(rcp.debugName, "Input Error -2, Sn %v, expect len %v, real len %v", seg.Sn, seg.Len, len(data))
			return -2
		}
		if seg.Cmd != IKCP_CMD_ACK && seg.Cmd != IKCP_CMD_WINS && seg.Cmd != IKCP_CMD_WASK && seg.Cmd != IKCP_CMD_PUSH {
			RcpDebugPrintf(rcp.debugName, "Input Error -3")
			return -3
		}
		rcp.rmtWnd = seg.Wnd
		rcp.sndBuf.ParseUna(seg.Una)
		rcp.ShrinkBuf() // TODO: check it if can be put back
		RcpDebugPrintf(rcp.debugName, "Input get Seg %v", seg.Sn)
		if seg.Cmd == IKCP_CMD_ACK {
			//if seg.Sn < rcp.sndUna {
			//	continue
			//}
			//if seg.Sn > rcp.sndUna {
			//	//println("hi")
			//}
			if rcp.current > seg.Ts {
				rcp.updateRto(rcp.current - seg.Ts)
			}
			if flag == 0 {
				flag = 1
				maxAck = seg.Sn
			} else if maxAck < seg.Sn {
				maxAck = seg.Sn
			}
			rcp.sndBuf.ParseAck(seg.Sn)
			rcp.ShrinkBuf()
			RcpDebugPrintf(rcp.debugName, "Input get ACK packet, sn %v , SndUna %v", seg.Sn, rcp.sndUna)
		} else if seg.Cmd == IKCP_CMD_PUSH {
			if seg.Sn == 1 {
				//println("hi")
			}

			if rcp.rcvNext+uint32(rcp.rcvWind) > seg.Sn {
				rcp.ackList = append(rcp.ackList, AckNode{seg.Sn, seg.Ts})
				//seg.Data = make([]byte, seg.Len)
				//copy(seg.Data, data)
				seg.Data = data[:seg.Len]
				rcp.rcvBuf.PushSegment(&seg)
				rcp.updateRcvQueue()
				RcpDebugPrintf(rcp.debugName, "Input get Push packet, sn %v, windows size %v", seg.Sn, rcp.getUnusedWindSize())
			} else {
				RcpDebugPrintf(rcp.debugName, "Recv Wind Full, sn %v lose", seg.Sn)
			}
		} else if seg.Cmd == IKCP_CMD_WASK {
			rcp.sendBitFlag |= IKCP_SEND_WIND_FLAG
		} else if seg.Cmd == IKCP_CMD_WINS {
			// have been processed before
		} else {
			return -3
		}
		data = data[seg.Len:]
		cnt += 1
	}
	if flag != 0 {
		rcp.sndBuf.ParseFastAck(maxAck)
	}
	if rcp.sndUna > prevUna {
		if rcp.cwnd < rcp.rmtWnd {
			mss := rcp.mss
			if rcp.cwnd < uint16(rcp.sshthresh) {
				rcp.cwnd++
				rcp.incr += uint16(mss)

			} else {
				if rcp.incr < uint16(mss) {
					rcp.incr = uint16(mss)
				}
				rcp.incr += (uint16(rcp.mss)*uint16(rcp.mss))/rcp.incr + (uint16(mss) / 16)
				if (rcp.cwnd+1)*uint16(rcp.mss) <= rcp.incr {
					rcp.cwnd++
				}
			}
			if rcp.cwnd > rcp.rmtWnd {
				rcp.cwnd = rcp.rmtWnd
				rcp.incr = rcp.rmtWnd * uint16(mss)
			}
		}
	}
	return 0
}

func (rcp *Rcpb) SetOutPut(outPutFun func(data []byte, size int)) {
	rcp.buffer.outputFun = outPutFun
}

func (rcp *Rcpb) updateRto(rtt uint32) {
	var rto uint32
	if rcp.RxSrtt == 0 {
		rcp.RxSrtt = rtt
		rcp.RxRttval = rtt / 2
	} else {
		delta := rtt - rcp.RxSrtt
		if rtt < rcp.RxSrtt {
			delta = rcp.RxSrtt - rtt
		}
		rcp.RxRttval = (3*rcp.RxRttval + delta) / 4
		rcp.RxSrtt = (7*rcp.RxSrtt + rtt) / 8
		if rcp.RxSrtt < 1 {
			rcp.RxSrtt = 1
		}
		maxBias := rcp.interval
		if maxBias < 4*rcp.RxRttval {
			maxBias = 4 * rcp.RxRttval
		}
		rto = rcp.RxSrtt + maxBias
		rcp.RxRto = rto
		if rcp.RxRto > IKCP_RTO_MAX {
			rcp.RxRto = IKCP_RTO_MAX
		}
		if rcp.RxRto < IKCP_RTO_MIN {
			rcp.RxRto = IKCP_RTO_MIN
		}

	}
}

func (rcp *Rcpb) getNextRecvPacketSize() int {
	if rcp.rcvQueue.Size() == 0 || rcp.rcvQueue.Size() < int(rcp.rcvQueue.Front().Seg.Frg) {
		return -1
	}
	seg := rcp.rcvQueue.Front().Seg
	if seg.Frg == 0 {
		return int(seg.Len)
	}

	var len uint32
	for p := rcp.rcvQueue.Front(); p != nil; p = p.Next {
		len += p.Seg.Len
		if p.Seg.Frg == 0 {
			break
		}
	}
	return 0
}

func (rcp *Rcpb) Recv(buffer []byte) int {
	if rcp.rcvQueue.Size() == 0 {
		return -1
	}
	nextPacketSize := rcp.getNextRecvPacketSize()
	if nextPacketSize < 0 {
		return -1
	}
	recoverFlag := false
	if rcp.rcvQueue.Size() >= int(rcp.rcvWind) {
		recoverFlag = true
	}

	packetFinishFlag := false
	for rcp.rcvQueue.Size() != 0 {
		p := rcp.rcvQueue.Front()
		seg := p.Seg

		copy(buffer, seg.Data)
		buffer = buffer[len(seg.Data):]
		rcp.rcvQueue.PopFront()
		if seg.Frg == 0 {
			packetFinishFlag = true
			break
		}
	}

	for rcp.rcvBuf.Size() > 0 {
		seg := rcp.rcvBuf.Front().Seg
		if seg.Sn == rcp.rcvNext && rcp.rcvQueue.Size() < int(rcp.rcvWind) {
			rcp.rcvBuf.PopFront()
			rcp.rcvQueue.PushSegment(seg)
			rcp.rcvNext++
		} else {
			break
		}
	}

	if rcp.rcvQueue.Size() < int(rcp.rcvWind) && recoverFlag {
		rcp.sendBitFlag |= IKCP_SEND_WIND_FLAG
	}

	if packetFinishFlag {
		return 0
	}
	return -2
}
