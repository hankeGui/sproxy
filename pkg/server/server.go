package server

import (
	"crypto/tls"
	"fmt"
	"github.com/iberryful/sproxy/pkg/log"
	"github.com/iberryful/sproxy/pkg/pipe"
	"io"
	"net"
	"time"
)

type ServerOption struct {
	KeyPath string
	CrtPath string
	Secret  string
	Listen  string
}

type Server struct {
	option *ServerOption
	crt    tls.Certificate
	secret string
}

func NewServer(o *ServerOption) (*Server, error) {
	s := &Server{
		option: o,
		secret: o.Secret,
	}
	cert, err := tls.LoadX509KeyPair(o.CrtPath, o.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("error creating server, %v", err)
	}
	s.crt = cert
	return s, nil
}

func (s *Server) Start() error {
	log.Infof("Listening at %s\n", s.option.Listen)
	l, err := tls.Listen("tcp", s.option.Listen, &tls.Config{Certificates: []tls.Certificate{s.crt}})
	if err != nil {
		return err
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Error(err)
			continue
		}

		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	p := pipe.New(conn, time.Duration(0))
	defer p.Close()
	for {
		// Noted that tcp keep alive message will return timeout when deadline is set.
		addr, err := p.WaitForHandShake(s.secret)

		if err == io.EOF {
			log.Errorf("[%s] pipe closed", p)
			return
		}

		if err != nil {
			log.Errorf("[%s] handshake error, %v", p, err)
			return
		}

		log.Infof("[%s] new connection %s", addr)
		tgt, err := net.Dial("tcp", addr.String())
		if err != nil {
			log.Errorf("[%s] connection %s failed, %s", p, addr, err)
			return
		}

		err = p.Bind(tgt)
		log.Infof("[%s] connection %s closed", p, addr)
		if err != nil {
			log.Debugf("%s pipe close, %s", p, err)
			return
		}
	}
}
