package mykcp

import "encoding/binary"

func ikcp_decode32u(p []byte, l *uint32) []byte {
	*l = binary.LittleEndian.Uint32(p)
	return p[4:]
}

func ikcp_decode16u(p []byte, l *uint16) []byte {
	*l = binary.LittleEndian.Uint16(p)
	return p[2:]
}

func ikcp_decode8u(p []byte, c *byte) []byte {
	*c = p[0]
	return p[1:]
}
