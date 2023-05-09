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
	data := make([]byte, 0)
	rawTransportInfo := make([]byte, 1500)
	size, _ := c.Read(rawTransportInfo)
	transportInfo := decodeTransportStruct(rawTransportInfo[:size])
	count := 0
	for count < transportInfo.Size {
		buf := make([]byte, 1500)
		size, _ := c.Read(buf)
		if size != 0 {
			data = append(data, buf[:size]...)
		}
		count += size
	}
	savePath := filepath.Join(s.downloadPath, transportInfo.FileName)
	log.Println("read done")
	err := os.WriteFile(savePath, data, 0644)
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
