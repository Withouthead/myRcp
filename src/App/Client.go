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
}

func (c *UploadDataClient) Init(remoteAddr string) {
	c.remoteAddr = remoteAddr
}

func (c *UploadDataClient) printSendInfo(conn *Rcp.RcpConn, fileName string, byteSum int64) {
	log.Printf("Send %v to %v\n", fileName, c.remoteAddr)
	bar := progressbar.Default(
		byteSum,
		"sending",
	)
	count := 0
	for count < int(byteSum) {
		_, size := conn.GetSendSpeedAndSendSum()
		count += int(size)
		bar.Add(int(size))
		<-time.After(1 * time.Second)
	}
}

func (c *UploadDataClient) SendFile(filePath string) {
	fileName := filepath.Base(filePath)
	b, _ := os.ReadFile(filePath)
	newByte := make([]byte, len(b))
	copy(newByte, b)
	data := encodeTransportStruct(TransportStruct{fileName, newByte})
	conn := Rcp.DialRcp(c.remoteAddr)
	conn.Write(data)
	conn.Close()
}
