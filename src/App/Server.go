package App

import (
	"Rcp/src/Rcp"
	"log"
	"os"
	"path/filepath"
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
	log.Println("get conn from client")
	data := make([]byte, 0)
	for !c.IsDead() {
		buf := make([]byte, 1500)
		size, err := c.Read(buf)
		if size != 0 {
			data = append(data, buf[:size]...)
		}
		if err == Rcp.READERROR_SUCCESS {
			break
		}
	}
	transportData := decodeTransportStruct(data)
	//filepath.Join()
	savePath := filepath.Join(s.downloadPath, transportData.FileName)
	err := os.WriteFile(savePath, transportData.Data, 0644)
	if err != nil {
		log.Fatalln(err)
	}
}

func (s *DownloadServer) Start(addr string, downloadPath string) {
	s.listener = Rcp.Listen(addr)
	s.downloadPath = downloadPath
	if _, err := os.Stat(s.downloadPath); err != nil {
		os.MkdirAll(s.downloadPath, os.ModePerm)
	}
	s.connectionCh = make(chan *Rcp.RcpConn)
	go s.accept()
}
