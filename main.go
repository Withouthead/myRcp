package main

import (
	"log"
	"strconv"
	"time"
)

func clientSend() {
	client := DialKcp("127.0.0.1:9666")
	cnt := 0
	for {
		if cnt > 5 {
			break
		}
		//log.Println(cnt)
		client.Write([]byte("hello" + strconv.Itoa(cnt)))
		cnt++
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(1 * time.Hour)
}
func main() {
	listener := Listen("127.0.0.1:9666")
	go clientSend()
	conn := listener.Accept()
	buf := make([]byte, 1024)
	for {
		size := conn.Read(buf)
		log.Println("Get " + string(buf[:size]))
	}

	time.Sleep(1 * time.Hour)
}
