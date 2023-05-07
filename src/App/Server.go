package App

import (
	"Rcp"
	"os"
)

type DownloadServer struct {
	listener     *Rcp.Listener
	downloadPath string
	connectionCh chan *Rcp.RcpConn
}

func (s *DownloadServer) accept() {
	c := s.listener.Accept()
	go s.startTask(c)
}

func (s *DownloadServer) startTask(c *Rcp.RcpConn) {
	data := make([]byte, 0)
	for !c.IsDead() {
		buf := make([]byte, 1500)
		size := c.Read(buf)
		data = append(data, buf[:size]...)
	}
	transportData := decodeTransportStruct(data)
	os.WriteFile(transportData.fileName, transportData.data, 0644)
}

func (s *DownloadServer) Start(addr string, downloadPath string) {
	s.listener = Rcp.Listen(addr)
	s.downloadPath = downloadPath
	s.connectionCh = make(chan *Rcp.RcpConn)
	go s.accept()
}
