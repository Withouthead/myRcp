package App

import (
	"bytes"
	"encoding/gob"
)

type TransportStruct struct {
	fileName string
	data     []byte
}

func decodeTransportStruct(data []byte) TransportStruct {
	var outputBuffer bytes.Buffer
	outputBuffer.Write(data)
	var transData TransportStruct
	gob.NewDecoder(&outputBuffer).Decode(&transData)
	return transData
}

func encodeTransportStruct(data TransportStruct) []byte {
	var inputBuffer bytes.Buffer
	gob.NewEncoder(&inputBuffer).Encode(&data)
	return inputBuffer.Bytes()
}
