package main

import (
	"log"
	"strconv"
	"time"
)

func clientSend() {
	client := DialRcp("127.0.0.1:9666")
	cnt := 0
	client.AddSendMask(1)
	time.Sleep(5 * time.Second)
	for {
		if cnt == 7 {
			client.ClearSendMask()
		}
		log.Printf("send %v", cnt)
		client.Write([]byte("hello" + strconv.Itoa(cnt)))
		cnt++
		time.Sleep(1 * time.Second)
	}
	time.Sleep(1 * time.Hour)
}
func main() {
	listener := Listen("127.0.0.1:9666")
	go clientSend()
	conn := listener.Accept()
	conn.SetRecvWindSize(4)
	buf := make([]byte, 1024)
	//time.Sleep(1 * time.Minute)
	for {
		size := conn.Read(buf)
		log.Println("Get " + string(buf[:size]))
	}

	time.Sleep(1 * time.Hour)
}
