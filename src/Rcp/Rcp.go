package Rcp

import (
	"crypto/rand"
	"encoding/binary"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type RcpConn struct {
	dead            int32
	kcpb            *Rcpb
	readUdpPacketCh chan []byte
	udpConn         net.PacketConn
	mu              sync.Mutex
	conectionErrCh  chan struct{}
	udpReadCh       chan []byte
	readEventCh     chan struct{}
	writeEventCh    chan struct{}
	die             chan struct{}
	remoteAddr      net.Addr
	isServer        bool
	debugName       string // ForDebug
}

func (conn *RcpConn) IsDead() bool {
	var flag int32
	atomic.LoadInt32(&flag)
	return flag > 0
}

func (conn *RcpConn) kcpOutPut(buf []byte, size int) {
	//conn.udpConn.WriteTo(buf, conn.remoteAddr)
	RcpDebugPrintf(conn.debugName, "Send packet size %v", size)
	_, err := conn.udpConn.WriteTo(buf[:size], conn.remoteAddr)
	if err != nil {
		log.Fatalf("send Error %v", err)
	} else {
		//log.Printf("send data, len %v", len(buf))
	}
}

func (conn *RcpConn) SetDead() {
	atomic.AddInt32(&conn.dead, 1)
}
func NewKcpConn(conv uint32, udpConn net.PacketConn, remoteAddr net.Addr, isServer bool) *RcpConn {

	kcpb := NewKcpB(conv, udpConn.LocalAddr().String())
	kcpConn := &RcpConn{}
	kcpConn.kcpb = kcpb
	kcpConn.udpConn = udpConn
	kcpConn.udpReadCh = make(chan []byte)
	kcpConn.readEventCh = make(chan struct{}, 1024) // TODO: make sure the chan size is ok
	kcpConn.writeEventCh = make(chan struct{})
	kcpConn.die = make(chan struct{})
	kcpConn.kcpb.SetOutPut(kcpConn.kcpOutPut)
	kcpConn.isServer = isServer
	kcpConn.debugName = "Connection " + udpConn.LocalAddr().String()
	if isServer {
		kcpConn.readUdpPacketCh = make(chan []byte, 5000) // TODO: make sure the chan size is ok
	}
	kcpConn.remoteAddr = remoteAddr
	go kcpConn.run()
	go kcpConn.readUdpData()
	return kcpConn
}

func (conn *RcpConn) readUdpData() {
	buffer := make([]byte, 5000)
	for {
		if conn.isServer {
			data := <-conn.readUdpPacketCh
			conn.udpReadCh <- data
		} else {
			n, _, err := conn.udpConn.ReadFrom(buffer)
			if err != nil {
				log.Fatalf("read udp error %v", err)
			}
			sendBuf := make([]byte, n)
			copy(sendBuf, buffer)
			conn.udpReadCh <- sendBuf
		}

	}
}

func getCurrentTime() uint32 {
	return uint32(time.Now().UnixMilli())
}

func (conn *RcpConn) input(data []byte) {
	conn.mu.Lock()
	prevNext := conn.kcpb.rcvNext
	conn.kcpb.Input(data)
	lastNext := conn.kcpb.rcvNext
	conn.mu.Unlock()

	//log.Print("data is ready to read by kcp Read function")
	if prevNext < lastNext {
		conn.readEventCh <- struct{}{}
	}

}

func (conn *RcpConn) run() {
	updateTime := 1 * time.Millisecond
	if !conn.isServer {
		//print("hi") // TODO: delete it
	}
	for !conn.IsDead() {
		conn.mu.Lock()
		conn.kcpb.Flush()
		conn.mu.Unlock()
		select {
		case data := <-conn.udpReadCh:
			conn.input(data)
		case <-time.After(updateTime):
			continue
		case <-conn.die:
			return
		}
	}
}
func (conn *RcpConn) Close() {
	close(conn.die)
	conn.mu.Lock()
	defer conn.mu.Unlock()
	conn.kcpb.Flush()
}

func (conn *RcpConn) Write(data []byte) {
	for {
		conn.mu.Lock()
		if conn.kcpb.sndWind > uint16(conn.kcpb.sndNext)-uint16(conn.kcpb.sndUna) {
			//log.Print("use write")
			flag := conn.kcpb.Send(data)
			if flag == 0 {
				conn.mu.Unlock()
				return
			}
		} else {
			RcpDebugPrintf(conn.debugName, "Send Wind Full, Send Error")
		}
		conn.mu.Unlock()
		select {
		case <-conn.die:
			return
		case <-time.After(1 * time.Millisecond):
			continue
		}
	}
}

func (conn *RcpConn) SetRecvWindSize(size uint16) {
	conn.kcpb.rcvWind = size
}

func (conn *RcpConn) AddSendMask(sn uint32) {
	conn.kcpb.SendMask[sn] = struct{}{}
}

func (conn *RcpConn) ClearSendMask() {
	conn.kcpb.SendMask = make(map[uint32]struct{})
}

func (conn *RcpConn) GetSendSpeedAndSendSum() uint32 {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	sum := conn.kcpb.sendByteSum
	conn.kcpb.ClearUpSendSpeedTime()
	return sum
}

const (
	READERROR_SUCCESS = iota
	READERROR_BUF_SIZE_NOT_ENOUGHt
	READERROR_FRG_NOT_FINISH
	READERROR_CONN_DIE
)

func (conn *RcpConn) Read(buf []byte) (int, int) {
	recvCount := 0
	for !conn.IsDead() {
		//log.Print("want to read something...")
		//RcpDebugPrintf(conn.debugName, "want to read something")
		conn.mu.Lock()
		size := len(buf)
		n := conn.kcpb.getNextRecvPacketSize()
		if n > 0 {
			if n > size {
				conn.mu.Unlock()
				return recvCount, READERROR_BUF_SIZE_NOT_ENOUGHt
			}

			flag := conn.kcpb.Recv(buf)
			if flag == 0 {
				recvCount += n
				conn.mu.Unlock()
				return recvCount, READERROR_SUCCESS
			} else if flag != -1 {
				recvCount += n
				buf = buf[n:]
			}
		}
		conn.mu.Unlock()
		select {
		case <-conn.readEventCh:
			continue
		case <-conn.die:
			return recvCount, READERROR_CONN_DIE
		}
	}
	return recvCount, READERROR_CONN_DIE
}

const ACCEPT_MAX_SIZE = 128

type Listener struct {
	udpConn    net.PacketConn
	connectMap map[string]*RcpConn
	chAccepts  chan *RcpConn
	die        chan struct{}
	debugName  string
}

func Listen(addr string) *Listener {
	udpAddr, _ := net.ResolveUDPAddr("udp", addr)
	udpConn, _ := net.ListenUDP("udp", udpAddr)
	l := &Listener{}
	l.udpConn = udpConn
	l.connectMap = make(map[string]*RcpConn)
	l.chAccepts = make(chan *RcpConn, ACCEPT_MAX_SIZE)
	l.die = make(chan struct{})
	l.debugName = "Listener " + udpConn.LocalAddr().String()
	go l.run()
	return l
}

func (l *Listener) Accept() *RcpConn {
	select {
	case <-l.die:
		return nil
	case c := <-l.chAccepts:
		return c
	}
}

type packet struct {
	Addr net.Addr
	data []byte
}

func (l *Listener) run() {
	chPacket := make(chan packet, 8192)
	go func() {
		data := make([]byte, 5000)
		for {
			n, addr, err := l.udpConn.ReadFrom(data)
			if err != nil {
				log.Fatalf("receive udp data error %v", err)
			}
			if n < IKCP_OVERHEAD {
				continue
			}
			p := packet{}
			p.Addr = addr
			p.data = make([]byte, n)
			copy(p.data, data)
			chPacket <- p
		}
	}()

loop:
	for {
		select {
		case p := <-chPacket:
			if len(p.data) < IKCP_OVERHEAD {
				continue
			}
			addrString := p.Addr.String()
			conv := binary.LittleEndian.Uint32(p.data)
			conn, ok := l.connectMap[addrString]
			if !ok {
				RcpDebugPrintf(l.debugName, "Accept NewConnection, addr %v", addrString)
				conn = NewKcpConn(conv, l.udpConn, p.Addr, true)
				l.connectMap[addrString] = conn
				l.chAccepts <- conn
				conn.readUdpPacketCh <- p.data
			} else {
				conn.readUdpPacketCh <- p.data // TODO: add timeout
			}
		case <-l.die:
			break loop
		}

	}
	l.udpConn.Close()
}

func DialRcp(addr string) *RcpConn {
	udpAddr, _ := net.ResolveUDPAddr("udp", addr)
	udpConn, _ := net.DialUDP("udp", nil, udpAddr)
	var conv uint32
	binary.Read(rand.Reader, binary.LittleEndian, &conv)
	kcpConn := NewKcpConn(conv, &ConnectedUDPConn{udpConn, udpConn}, udpAddr, false)
	return kcpConn
}

type ConnectedUDPConn struct {
	*net.UDPConn
	Conn net.Conn
}

func (c *ConnectedUDPConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	return c.Write(b)
}
