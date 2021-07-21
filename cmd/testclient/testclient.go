package main

import (
	"flag"
	"golang.org/x/net/proxy"
	"log"
	"net"
	"time"
)

var (
	remoteAddr string
	proxyAddr  string
	halfClose  bool
)

func init() {
	flag.StringVar(&remoteAddr, "l", "127.0.0.1:5678", "remote address")
	flag.StringVar(&proxyAddr, "p", "", "remote address")
	flag.BoolVar(&halfClose, "h", true, "half close")
	flag.Parse()
	log.SetFlags(log.Lmicroseconds)
}

func main() {
	log.Printf("connecting %s", remoteAddr)
	conn := connect()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf[:10])
	if err != nil {
		log.Println(err)
	}
	log.Printf("read %d bytes", n)
	log.Println(halfClose)
	if !halfClose {
		log.Println("close")
		conn.Close()
		return
	}

	log.Println("half close(write)")
	conn.(*net.TCPConn).CloseWrite()
	for {
		dn, err := conn.Read(buf)
		n += dn
		if err != nil {
			log.Printf("read %d bytes", n)
			log.Println("close")
			conn.Close()
			time.Sleep(2 * time.Second)
			log.Println("quit")
			return
		}
	}

}

func connect() net.Conn {
	if len(proxyAddr) == 0 {
		log.Println("DIRECT")
		conn, err := net.Dial("tcp", remoteAddr)
		if err != nil {
			log.Fatal(err)
		}
		return conn
	} else {
		log.Printf("SOCKS5 %s", proxyAddr)
		dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, &net.Dialer{})
		if err != nil {
			log.Fatal(err)
		}
		conn, err := dialer.Dial("tcp", remoteAddr)
		if err != nil {
			log.Fatal(err)
		}
		return conn
	}
}
