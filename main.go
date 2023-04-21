package main

func main() {
	k := DialKcp(":300")
	s := "test"
	b := []byte(s)
	k.Write(b)
}
