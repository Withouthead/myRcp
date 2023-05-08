package main

import (
	"Rcp/src/App"
	"time"
)

func StartServer() {
	server := App.DownloadServer{}
	server.Start("127.0.0.1:9666", "./download")

}

func main() {
	go StartServer()
	client := App.UploadDataClient{}
	client.Init("127.0.0.1:9666")
	client.SendFile("test.txt")
	time.Sleep(1 * time.Hour)
}
