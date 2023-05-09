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
	b, _ := os.ReadFile(filePath)
	transportInfoData := encodeTransportStruct(TransportInfo{fileName, len(b)})
	conn := Rcp.DialRcp(c.remoteAddr)
	go c.printSendInfo(conn, fileName, int64(len(b)))
	conn.Write(transportInfoData)
	size := len(b)
	for size > 0 {
		if size < c.sliceSize {
			conn.Write(b)
		} else {
			conn.Write(b[:c.sliceSize])
			b = b[c.sliceSize:]
		}
		size = len(b)
	}
	conn.Write(b)
	conn.Close()
}
