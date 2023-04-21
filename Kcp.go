package main

import (
	"crypto/rand"
	"encoding/binary"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type KcpConn struct {
	dead            int32
	kcpb            *KCPB
	readUdpPacketCh chan []byte
	udpConn         net.PacketConn
	mu              sync.Mutex
	conectionErrCh  chan struct{}
	udpReadCh       chan []byte
	readEventCh     chan struct{}
	writeEventCh    chan struct{}
	die             chan struct{}
	remoteAddr      net.Addr
}

func (conn *KcpConn) IsDead() bool {
	var flag int32
	atomic.LoadInt32(&flag)
	return flag > 0
}

func (conn *KcpConn) kcpOutPut(buf []byte) {
	//conn.udpConn.WriteTo(buf, conn.remoteAddr)
	_, err := conn.udpConn.WriteTo(buf, conn.remoteAddr)
	if err != nil {
		log.Fatalf("send Error %v", err)
	} else {
		log.Printf("send data, len %v", len(buf))
	}
}

func (conn *KcpConn) SetDead() {
	atomic.AddInt32(&conn.dead, 1)
}
func NewKcpConn(conv uint32, udpConn net.PacketConn, remoteAddr net.Addr) *KcpConn {

	kcpb := NewKcpB(conv)
	kcpConn := &KcpConn{}
	kcpConn.kcpb = kcpb
	kcpConn.udpConn = udpConn
	kcpConn.udpReadCh = make(chan []byte)
	kcpConn.readEventCh = make(chan struct{}, 1024) // TODO: make sure the chan size is ok
	kcpConn.writeEventCh = make(chan struct{})
	kcpConn.kcpb.SetOutPut(kcpConn.kcpOutPut)
	kcpConn.readUdpPacketCh = make(chan []byte, 1024) // TODO: make sure the chan size is ok
	kcpConn.remoteAddr = remoteAddr
	go kcpConn.run()
	go kcpConn.readUdpData()
	return kcpConn
}

func (conn *KcpConn) readUdpData() {
	for {
		data := <-conn.readUdpPacketCh
		conn.udpReadCh <- data
	}
}

func getCurrentTime() uint32 {
	return uint32(time.Now().UnixMilli())
}

func (conn *KcpConn) input(data []byte) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	conn.kcpb.Input(data)
	log.Print("data is ready to read by kcp Read function")
	conn.readEventCh <- struct{}{}
}

func (conn *KcpConn) run() {
	updateTime := 10 * time.Millisecond
	if conn.remoteAddr.String() != ":9666" {
		print("hi")
	}
	for !conn.IsDead() {
		conn.mu.Lock()
		conn.kcpb.Update(getCurrentTime())

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

func (conn *KcpConn) Write(data []byte) {
	conn.mu.Lock()
	if conn.kcpb.sndWind > uint16(conn.kcpb.sndNext)-uint16(conn.kcpb.sndUna) {
		flag := conn.kcpb.Send(data)
		if flag == 0 {
			conn.mu.Unlock()
			return
		}
	}
	conn.mu.Unlock()
}

func (conn *KcpConn) Read(buf []byte) {
	for !conn.IsDead() {
		log.Print("want to read something...")
		conn.mu.Lock()
		size := len(buf)
		readFlag := false
		for {
			n := conn.kcpb.getNextRecvPacketSize()
			if n <= 0 || n > size || size <= 0 {
				break
			}
			flag := conn.kcpb.Recv(buf)
			if flag != 0 {
				break
			}
			size -= n
			readFlag = true
		}
		if readFlag {
			conn.mu.Unlock()
			return
		}
		conn.mu.Unlock()
		select {
		case <-conn.readEventCh:
			continue
		case <-conn.die:
			return
		}
	}
}

const ACCEPT_MAX_SIZE = 128

type Listener struct {
	udpConn    net.PacketConn
	connectMap map[string]*KcpConn
	chAccepts  chan *KcpConn
	die        chan struct{}
}

func Listen(addr string) *Listener {
	udpAddr, _ := net.ResolveUDPAddr("udp", addr)
	udpConn, _ := net.ListenUDP("udp", udpAddr)
	l := &Listener{}
	l.udpConn = udpConn
	l.connectMap = make(map[string]*KcpConn)
	l.chAccepts = make(chan *KcpConn, ACCEPT_MAX_SIZE)
	l.die = make(chan struct{})
	go l.run()
	return l
}

func (l *Listener) Accept() *KcpConn {
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
		for {
			data := make([]byte, 1500)
			n, addr, err := l.udpConn.ReadFrom(data)
			if err != nil {
				log.Fatalf("receive udp data error %v", err)
			}
			if n < IKCP_OVERHEAD {
				continue
			}
			chPacket <- packet{addr, data[:n]}
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
				conn = NewKcpConn(conv, l.udpConn, p.Addr)
				l.connectMap[addrString] = conn
				l.chAccepts <- conn
			} else {
				conn.readUdpPacketCh <- p.data // TODO: add timeout
			}
		case <-l.die:
			break loop
		}

	}
	l.udpConn.Close()
}

func DialKcp(addr string) *KcpConn {
	udpAddr, _ := net.ResolveUDPAddr("udp", addr)
	udpConn, _ := net.DialUDP("udp", nil, udpAddr)
	var conv uint32
	binary.Read(rand.Reader, binary.LittleEndian, &conv)
	kcpConn := NewKcpConn(conv, &ConnectedUDPConn{udpConn, udpConn}, udpAddr)
	return kcpConn
}

type ConnectedUDPConn struct {
	*net.UDPConn
	Conn net.Conn
}

func (c *ConnectedUDPConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	return c.Write(b)
}
