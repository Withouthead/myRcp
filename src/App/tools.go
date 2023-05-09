package App

import (
	"bytes"
	"encoding/gob"
	"log"
)

type TransportInfo struct {
	FileName string
	Size     int
}

func decodeTransportStruct(data []byte) TransportInfo {
	var outputBuffer bytes.Buffer
	outputBuffer.Write(data)
	var transData TransportInfo
	err := gob.NewDecoder(&outputBuffer).Decode(&transData)
	if err != nil {
		log.Fatalln(err)
	}
	return transData
}

func encodeTransportStruct(data TransportInfo) []byte {
	var inputBuffer bytes.Buffer
	err := gob.NewEncoder(&inputBuffer).Encode(&data)
	if err != nil {
		log.Fatalln(err)
	}
	return inputBuffer.Bytes()
}
