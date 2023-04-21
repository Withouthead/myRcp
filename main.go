package main

import "time"

func main() {
	listener := Listen("127.0.0.1:9666")
	client := DialKcp("127.0.0.1:9666")
	client.Write([]byte("hello"))
	conn := listener.Accept()
	buf := make([]byte, 1024)
	conn.Read(buf)
	println(string(buf))
	time.Sleep(1 * time.Hour)
}
