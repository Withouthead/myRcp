package main

import (
	"Rcp/src/App"
	"crypto/md5"
	"fmt"
	"io"
	_ "net/http/pprof"
	"os"
	"path/filepath"
)

func StartServer(downloadPath string) {
	server := App.DownloadServer{}
	server.Start("127.0.0.1:9666", downloadPath)

}

func getMd5Str(filePath string) string {
	sourceFile, _ := os.Open(filePath)
	sourceMd5Handle := md5.New()               //创建 md5 句柄
	io.Copy(sourceMd5Handle, sourceFile)       //将文件内容拷贝到 md5 句柄中
	sourceMd := sourceMd5Handle.Sum(nil)       //计算 MD5 值，返回 []byte
	sourceMdStr := fmt.Sprintf("%x", sourceMd) //将 []byte 转为 string
	return sourceMdStr
}
func main() {
	downloadPath := "./download"
	filePath := "./test.txt"
	fileName := filepath.Base(filePath)
	go StartServer(downloadPath)
	client := App.UploadDataClient{}
	client.Init("127.0.0.1:9666")
	isDone := client.SendFile(filePath)
	if isDone {
		println("calculate md5")
		sourceMdStr := getMd5Str(filePath)
		desMdStr := getMd5Str(filepath.Join(downloadPath, fileName))
		fmt.Printf("source File Md5 is %v, download File Md5 is %v", sourceMdStr, desMdStr)
	}
}
