package App

import (
	"Rcp/src/Rcp"
	"os"
	"path/filepath"
	"time"
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
	savePath := filepath.Join(s.downloadPath, transportInfo.FileName)

	var f *os.File
	if checkFileIsExist(savePath) {
		f, _ = os.Create(savePath)
	} else {
		f, _ = os.OpenFile(savePath, os.O_APPEND, 0666)
	}

	for count < transportInfo.Size {
		buf := make([]byte, 1500)
		size, _ := c.Read(buf)
		if size != 0 {
			data = append(data, buf[:size]...)
			if len(data) > 1024*1024*5 {
				f.Write(data)
				data = make([]byte, 0)
			}
		}
		count += size
	}
	//log.Println("read done")
	if len(data) > 0 {
		f.Write(data)
	}
	time.Sleep(5 * time.Second)
}

func checkFileIsExist(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
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
