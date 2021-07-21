package main

import (
	"crypto/rand"
	"flag"
	"log"
	"net"
	"time"
)

var (
	listenAddr string
	delayTime  time.Duration
	nBytes     int
)

func init() {
	flag.StringVar(&listenAddr, "l", "0.0.0.0:5678", "listen address")
	flag.DurationVar(&delayTime, "t", 100*time.Millisecond, "delay time")
	flag.IntVar(&nBytes, "n", 1024*1024, "n bytes")
	flag.Parse()
	log.SetFlags(log.Lmicroseconds)
}

func main() {
	log.Printf("listening at %s", listenAddr)
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	buf := make([]byte, nBytes)
	time.Sleep(delayTime)
	rand.Read(buf)
	n, err := conn.Write(buf)
	if err != nil {
		log.Println(err)
	}
	log.Printf("write %d bytes to %s", n, conn.RemoteAddr())
	conn.Close()
}
