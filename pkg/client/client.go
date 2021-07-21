package client

import (
	"crypto/tls"
	"github.com/iberryful/sproxy/pkg/log"
	"net"
	"sync/atomic"
	"time"

	"github.com/iberryful/sproxy/pkg/pipe"
	"github.com/iberryful/sproxy/pkg/socks"
)

type RemoteConn struct {
	t    time.Time
	conn net.Conn
}

type ClientOption struct {
	ListenAddr string
	RemoteAddr string
	Secret     string
	PoolSize   int
	Timeout    time.Duration
}

type Client struct {
	Option      *ClientOption
	Pool        *pipe.Pool
	activeCount int64
}

func New(o *ClientOption) *Client {
	c := &Client{
		Option: o,
	}
	c.Pool = pipe.NewPool(o.PoolSize, c.Option.Timeout, c.Dial)
	return c
}

func (c *Client) ListenAndServe() error {
	log.Infof("listening at %s", c.Option.ListenAddr)
	log.Infof("Pool size: %d, timeout: %s", c.Option.PoolSize, c.Option.Timeout)
	l, err := net.Listen("tcp", c.Option.ListenAddr)
	if err != nil {
		return err
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Error(err)
			continue
		}
		go c.handleConn(conn)
	}
}

func (c *Client) handleConn(conn net.Conn) error {
	defer conn.Close()
	t := time.Now()
	addr, err := socks.Handshake(conn)
	if err != nil {
		log.Error(err)
		return nil
	}

	var p *pipe.Pipe

	for {
		p, err = c.Pool.Get()
		if err != nil {
			return err
		}
		err = p.TryPing()
		if err != nil {
			log.Warnf("[%s] ping failed, %s", p, err)
			p.Close()
			continue
		}
		break
	}

	err = p.HandShake(addr, c.Option.Secret)
	if err != nil {
		log.Warnf("[%s] error handshake, %s", p, err)
		p.Close()
		return nil
	}

	atomic.AddInt64(&c.activeCount, 1)
	log.Infof("[%s] [conn: %2d] [pool: %2d] handle conn: %s, handshake time: %d ms", p, c.activeCount, c.Pool.Len(), addr, time.Now().Sub(t).Milliseconds())
	err = p.Bind(conn)
	atomic.AddInt64(&c.activeCount, -1)
	if err == nil {
		c.Pool.Put(p)
		log.Infof("[%s] [conn: %2d] [pool: %2d] %s closed", p, c.activeCount, c.Pool.Len(), addr)
		return nil
	}

	p.Close()
	log.Infof("[%s] [conn: %2d] [pool: %2d] %s closed", p, c.activeCount, c.Pool.Len(), addr)
	return nil
}

func (c *Client) Dial() (*pipe.Pipe, error) {
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	r, err := tls.Dial("tcp", c.Option.RemoteAddr, conf)

	if err != nil {
		return nil, err
	}
	return pipe.New(r, c.Option.Timeout), nil
}
