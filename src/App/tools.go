package App

import (
	"bytes"
	"encoding/gob"
	"log"
)

type TransportStruct struct {
	FileName string
	Data     []byte
}

func decodeTransportStruct(data []byte) TransportStruct {
	var outputBuffer bytes.Buffer
	outputBuffer.Write(data)
	var transData TransportStruct
	err := gob.NewDecoder(&outputBuffer).Decode(&transData)
	if err != nil {
		log.Fatalln(err)
	}
	return transData
}

func encodeTransportStruct(data TransportStruct) []byte {
	var inputBuffer bytes.Buffer
	err := gob.NewEncoder(&inputBuffer).Encode(&data)
	if err != nil {
		log.Fatalln(err)
	}
	return inputBuffer.Bytes()
}
