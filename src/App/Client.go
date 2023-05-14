package App

import (
	"Rcp/src/Rcp"
	"github.com/schollz/progressbar/v3"
	"log"
	"os"
	"path/filepath"
	"time"
)

type UploadDataClient struct {
	remoteAddr string
	sliceSize  int
}

func (c *UploadDataClient) Init(remoteAddr string) {
	c.remoteAddr = remoteAddr
	c.sliceSize = 1024 * 100
}

func (c *UploadDataClient) printSendInfo(conn *Rcp.RcpConn, fileName string, byteSum int64) {
	log.Printf("Send %v to %v\n", fileName, c.remoteAddr)
	bar := progressbar.DefaultBytes(
		byteSum,
		"sending",
	)
	count := 0
	for count < int(byteSum) {
		size := int(conn.GetSendSpeedAndSendSum())
		//size = 1
		if size > int(byteSum)-count {
			size = int(byteSum) - count
		}
		count += size

		bar.Add(size)
		<-time.After(1 * time.Second)
	}
}

func (c *UploadDataClient) SendFile(filePath string) {
	fileName := filepath.Base(filePath)
	f, _ := os.Open(filePath)
	defer f.Close()
	fileStat, _ := f.Stat()
	fileSize := fileStat.Size()
	transportInfoData := encodeTransportStruct(TransportInfo{fileName, int(fileSize)})
	conn := Rcp.DialRcp(c.remoteAddr)
	go c.printSendInfo(conn, fileName, fileSize)
	conn.Write(transportInfoData)
	readBuf := make([]byte, 5*1024*10)
	count := 0
	for {
		n, _ := f.Read(readBuf)
		if n == 0 {
			break
		}
		count += n
		conn.Write(readBuf[:n])
	}
	log.Println("send Finish count %v, file size %v", count, fileSize)
	time.Sleep(5 * time.Second)
	conn.Close()
}
