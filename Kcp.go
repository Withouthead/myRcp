package main

import (
	"crypto/rand"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type KcpConn struct {
	dead           int32
	kcpb           *KCPB
	udpConn        *net.UDPConn
	mu             sync.Mutex
	conectionErrCh chan struct{}
	udpReadCh      chan []byte
	readEventCh    chan struct{}
	writeEventCh   chan struct{}
	die            chan struct{}
	remoteAddr     net.Addr
}

func (conn *KcpConn) IsDead() bool {
	var flag int32
	atomic.LoadInt32(&flag)
	return flag > 0
}

func (conn *KcpConn) kcpOutPut(buf []byte) {
	conn.udpConn.WriteTo(buf, conn.remoteAddr)
}

func (conn *KcpConn) SetDead() {
	atomic.AddInt32(&conn.dead, 1)
}
func NewKcpConn(conv uint32, udpConn *net.UDPConn, remoteAddr net.Addr) *KcpConn {

	kcpb := NewKcpB(conv)
	kcpConn := &KcpConn{}
	kcpConn.kcpb = kcpb
	kcpConn.udpConn = udpConn
	kcpConn.udpReadCh = make(chan []byte)
	kcpConn.readEventCh = make(chan struct{})
	kcpConn.writeEventCh = make(chan struct{})
	kcpConn.kcpb.SetOutPut(kcpConn.kcpOutPut)
	go kcpConn.run()
	go kcpConn.readUdpData()
	return kcpConn
}

func (conn *KcpConn) readUdpData() {
	for {
		data := make([]byte, 0)
		n, _, err := conn.udpConn.ReadFrom(data)
		if err != nil {
			conn.conectionErrCh <- struct{}{}
			return
		}
		conn.udpReadCh <- data[:n]
	}
}

func getCurrentTime() uint32 {
	return uint32(time.Now().UnixMilli())
}

func (conn *KcpConn) input(data []byte) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	conn.kcpb.Input(data)
	conn.readEventCh <- struct{}{}
}

func (conn *KcpConn) run() {
	updateTime := 10 * time.Millisecond
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
			return
		}
	}
	conn.mu.Unlock()
}

func (conn *KcpConn) Read(buf []byte) {
	for !conn.IsDead() {
		conn.mu.Lock()
		flag := conn.kcpb.Recv(buf)
		if flag == 0 && len(buf) > 0 {
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

type Listener struct {
	udpConn    *net.UDPConn
	connectMap map[string]*KcpConn
	chAccpets  chan *KcpConn
	die        chan struct{}
}

func (l *Listener) Accept() *KcpConn {
	select {
	case <-l.die:
		return nil
	case c := <-l.chAccpets:
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
			data := make([]byte, 0)
			n, addr, _ := l.udpConn.ReadFrom(data)
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
				l.chAccpets <- conn
			} else {
				//TODO: it
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
	kcpConn := NewKcpConn(conv, udpConn, udpAddr)
	return kcpConn
}
