package main

import (
	"Rcp/src/App"
	"github.com/sasha-s/go-deadlock"
	"time"
)

func StartServer() {
	server := App.DownloadServer{}
	server.Start("127.0.0.1:9666", "./download")

}

func main() {
	deadlock.Opts.DeadlockTimeout = time.Second
	go StartServer()
	client := App.UploadDataClient{}
	client.Init("127.0.0.1:9666")
	client.SendFile("test.txt")
	time.Sleep(1 * time.Hour)
}
