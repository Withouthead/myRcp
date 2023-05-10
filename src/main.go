package main

import (
	"Rcp/src/App"
	"github.com/sasha-s/go-deadlock"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"
)

func StartServer() {
	server := App.DownloadServer{}
	server.Start("127.0.0.1:9666", "./download")

}

func main() {
	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()
	deadlock.Opts.DeadlockTimeout = time.Second
	go StartServer()
	client := App.UploadDataClient{}
	client.Init("127.0.0.1:9666")
	client.SendFile("test.txt")
	time.Sleep(1 * time.Hour)
}
