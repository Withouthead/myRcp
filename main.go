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
		log.Println(cnt)
		client.Write([]byte("hello" + strconv.Itoa(cnt)))
		cnt++
		time.Sleep(10 * time.Millisecond)
	}
}
func main() {
	listener := Listen("127.0.0.1:9666")
	go clientSend()
	conn := listener.Accept()
	buf := make([]byte, 1024)
	for {
		size := conn.Read(buf)
		println(string(buf[:size]))
	}

	time.Sleep(1 * time.Hour)
}
