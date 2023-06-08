package App

import (
	"Rcp/src/Rcp"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"gonum.org/v1/plot"
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

func (c *UploadDataClient) plotPic(ch chan int) {
	lastTime := time.Now().UnixMilli()
	sum := 0
	p := plot.New()
	p.Title.Text = "正常网络测试"
	y := []float64{}
	for b := range ch {
		if b == -1 {
			break
		}
		if time.Now().UnixMilli()-lastTime > 1000 {
			y = append(y, float64(sum)/(1024.0*1024.0))
			sum = 0
		}
		sum += b
	}
	f, _ := os.Create("./speed.txt")
	for _, v := range y {
		fmt.Fprintf(f, "%v\n", v)
	}
	f.Close()
}

func (c *UploadDataClient) printSendInfo(conn *Rcp.RcpConn, fileName string, byteSum int64) {
	log.Printf("Send %v to %v\n", fileName, c.remoteAddr)
	bar := progressbar.DefaultBytes(
		byteSum,
		"sending",
	)
	ch := make(chan int, 5000)
	go c.plotPic(ch)
	count := 0
	for count < int(byteSum) {
		size := int(conn.GetSendSpeedAndSendSum())
		//size = 1
		if size > 0 {
			ch <- size
		}
		if size > int(byteSum)-count {
			size = int(byteSum) - count
		}
		count += size
		bar.Add(size)

		<-time.After(1 * time.Second)
	}
	ch <- -1
}

func (c *UploadDataClient) SendFile(filePath string) bool {
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
	time.Sleep(5 * time.Second)
	log.Printf("send Finish count %v, file size %v", count, fileSize)
	conn.Close()
	return true
}
