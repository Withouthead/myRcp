package Rcp

import (
	"encoding/binary"
	"fmt"
	"log"
)

const DebugMode = false

func RcpDebugPrintf(addr string, format string, v ...interface{}) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	//if addr == "Block 127.0.0.1:9666" {
	//	return
	//} // TODO: delete it
	if DebugMode {
		log.Printf(fmt.Sprintf("[%v]: ", addr)+format, v...)
	}
}

func KcpErrorPrintf(addr string, format string, v ...interface{}) {
	log.Fatalf(fmt.Sprintf("[%v]: ", addr)+format, v...)
}

func ikcp_decode32u(p []byte, l *uint32) []byte {
	*l = binary.LittleEndian.Uint32(p)
	return p[4:]
}

func ikcp_decode16u(p []byte, l *uint16) []byte {
	*l = binary.LittleEndian.Uint16(p)
	return p[2:]
}

func ikcp_decode8u(p []byte, c *uint8) []byte {
	*c = p[0]
	return p[1:]
}

func ikcp_encode32u(p []byte, l uint32) []byte {
	binary.LittleEndian.PutUint32(p, l)
	return p[4:]
}

func ikcp_encode16u(p []byte, l uint16) []byte {
	binary.LittleEndian.PutUint16(p, l)
	return p[2:]
}

func ikcp_encode8u(p []byte, l uint8) []byte {
	p[0] = l
	return p[1:]
}
