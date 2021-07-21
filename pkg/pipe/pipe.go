package pipe

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/iberryful/sproxy/pkg/log"
	"github.com/iberryful/sproxy/pkg/socks"
)

const bufSize int = 16 * 1024
const HeaderLen int = 4
const MagicLen int = 4
const MaxLen = bufSize - HeaderLen - MagicLen

var Magic = []byte{0xff, 0x86, 0x13, 0x85}

const (
	CmdClose = iota //0
	CmdConn         //1
	CmdTrans        //2
	CmdPing         //3
	CmdErr          //4
)

const (
	Idle        = iota // 0
	InUse              // 1
	Interrupted        // 2
	Closed             // 3
)

var ErrInterrupted = errors.New("pipe interrupted")
var ErrPipe = errors.New("pipe closed")
var scn uint32 = 0

type Pipe struct {
	conn       net.Conn
	state      int64
	readBuf    []byte
	writeBuf   []byte
	n          int
	lastActive time.Time
	timeout    time.Duration
	id         uint32
	term       uint8
}

func New(conn net.Conn, timeout time.Duration) *Pipe {
	p := &Pipe{
		conn:       conn,
		state:      Idle,
		readBuf:    make([]byte, bufSize),
		writeBuf:   make([]byte, bufSize),
		timeout:    timeout,
		lastActive: time.Now(),
		id:         atomic.AddUint32(&scn, 1),
	}
	copy(p.writeBuf, Magic)
	return p
}

func (p *Pipe) checkMagic() error {
	_, err := io.ReadFull(p.conn, p.readBuf[:len(Magic)])
	if err != nil {
		return err
	}
	if !bytes.Equal(Magic, p.readBuf[:len(Magic)]) {
		return fmt.Errorf("invalid magic, %x", p.readBuf[:len(Magic)])
	}

	return nil
}

func (p *Pipe) interruptLocal() bool {
	return atomic.CompareAndSwapInt64(&p.state, InUse, Interrupted)
}

func (p *Pipe) setDeadLine() {
	t := time.Time{}
	if p.timeout != time.Duration(0) {
		t = time.Now().Add(p.timeout)
	}
	//log.Printf("%s set deadline %s", p, t)
	p.conn.SetDeadline(t)
}

func (p *Pipe) String() string {
	return fmt.Sprintf("pid %04x, T%d", p.id&0xffff, p.term)
}

func (p *Pipe) Close() error {
	if p.state == Closed {
		return nil
	}
	p.setState(Closed)
	return p.conn.Close()
}

func (p *Pipe) setState(state int64) {
	atomic.StoreInt64(&p.state, state)
}

func (p *Pipe) reset() error {
	p.setDeadLine()
	p.term += 1
	p.setState(Idle)
	return nil
}

func (p *Pipe) Read(b []byte) (n int, err error) {
	// not in progress
	p.setDeadLine()
	for p.n == 0 {
		err := p.checkMagic()
		if err != nil {
			return 0, err
		}

		_, err = io.ReadFull(p.conn, p.readBuf[:4])
		if err != nil {
			return 0, err
		}

		cmd, term, length := p.readBuf[0], p.readBuf[1], int(p.readBuf[2])*256+int(p.readBuf[3])
		if cmd < CmdErr && term != p.term {
			if length > 0 {
				_, err = io.ReadFull(p.conn, p.readBuf[4:])
				if err != nil {
					return 0, err
				}
			}
			log.Warnf("[%s] ignore frame, term: %d, cmd: %d", p, term, cmd)
			continue
		}
		switch cmd {
		case CmdClose:
			return 0, ErrInterrupted
		case CmdConn:
			_, err = io.ReadFull(p.conn, p.readBuf[4:4+length])
			if err != nil {
				return 0, err
			}

			copy(b, p.readBuf[:4+length])
			return 4 + length, nil
		case CmdTrans:
			p.n = length
		case CmdPing:
			continue
		default:
			err := fmt.Errorf("[%s] unknown cmd: %d", p, cmd)
			log.Error(err)
			return 0, err
		}
	}

	nBytes := min(len(b), p.n)

	n, err = p.conn.Read(b[:nBytes])
	p.n -= n

	return n, err
}

func (p *Pipe) Write(b []byte) (n int, err error) {
	p.setDeadLine()
	n = len(b)
	m := len(Magic)
	pos := 0
	for n > 0 {
		nBytes := min(n, MaxLen)
		copy(p.writeBuf[m:], []byte{CmdTrans, p.term, byte(nBytes >> 8), byte(nBytes % 256)})
		copy(p.writeBuf[m+4:], b[pos:pos+nBytes])

		if _, err = p.conn.Write(p.writeBuf[:m+4+nBytes]); err != nil {
			return 0, err
		}

		pos += nBytes
		n -= nBytes
	}

	return len(b), nil
}

// reading from p to conn
func (p *Pipe) readLoop(conn net.Conn) error {
	_, err := io.Copy(conn, p)

	// p read closed, should close p
	if !IsInterrupted(err) {
		log.Debugf("[%s] [readLoop] [pipe close] %s", p, err)
		conn.Close()
		return ErrPipe
	}

	// no new data from p, won't write any data to conn
	log.Debugf("[%s] [readLoop] interrupted by remote", p)
	conn.SetDeadline(time.Now())
	conn.(*net.TCPConn).CloseWrite()
	//log.Printf("%s read loop error: %s", p, err)
	return nil
}

// reading from conn to pipe
func (p *Pipe) writeLoop(conn net.Conn) error {
	_, err := io.Copy(p, conn)

	if p.IsPipeErr(err) {
		conn.Close()
		return err
	}

	if p.interruptLocal() {
		if err := p.TryInterruptRemote(); err != nil {
			return err
		}
	}

	// no new data for p, sent interrupt to remote, won't read from conn
	if err == nil || IsReadError(err) {
		log.Debugf("[%s] [writeLoop] interrupted by local", p)
		// Normally, remote should send FIN signal, setting deadline here is just to make sure the read loop can exit.
		p.conn.SetDeadline(time.Now().Add(1 * time.Second))
		conn.(*net.TCPConn).CloseRead()
		return nil
	}

	return err
}

func (p *Pipe) Bind(conn net.Conn) error {
	var readLoopErr, writeLoopErr error
	ch := make(chan error, 1)
	defer close(ch)
	defer conn.Close()
	go func() {
		err := p.readLoop(conn)
		log.Debugf("[%s] [readLoop] exited, %v", p, err)
		ch <- err
	}()
	writeLoopErr = p.writeLoop(conn)
	log.Debugf("[%s] [writeLoop] exited, %v", p, writeLoopErr)
	readLoopErr = <-ch

	if p.IsPipeErr(readLoopErr) || p.IsPipeErr(writeLoopErr) {
		return ErrPipe
	}
	return p.reset()
}

func (p *Pipe) HandShake(addr socks.Addr, secret string) error {
	p.setState(InUse)
	buf := make([]byte, 1024)
	addrLen := len(addr)
	headerLen := addrLen + 32
	n := 4 + addrLen
	buf[0] = CmdConn
	buf[1] = p.term
	buf[2] = uint8(headerLen >> 8)
	buf[3] = uint8(headerLen % 256)
	copy(buf[4:], addr)

	hmac := sha256.Sum256(append(buf[:n], []byte(secret)...))
	copy(buf[n:], hmac[:])
	return p.writeCmd(buf[:n+32])
}

func (p *Pipe) WaitForHandShake(secret string) (socks.Addr, error) {
	p.setState(InUse)
	buf := make([]byte, 1024)
	var err error
	n, err := p.Read(buf)
	if err != nil || n < 4 {
		return socks.Addr{}, err
	}

	if buf[0] != CmdConn {
		return socks.Addr{}, fmt.Errorf("invalid handshake cmd: %d", buf[0])
	}

	length := 256*int(buf[2]) + int(buf[3])
	// n is the frame header without hmac
	n = 4 + length - 32

	msg := make([]byte, n+len(secret))
	copy(msg[:n], buf[:n])
	copy(msg[n:n+len(secret)], secret)

	hmac := sha256.Sum256(msg)

	if !bytes.Equal(hmac[:], buf[n:n+32]) {
		return socks.Addr{}, fmt.Errorf("invalid handshake hmac, expect %x, got %x", hmac, buf[n:n+32])
	}

	return buf[4:n], nil
}

func (p *Pipe) TryInterruptRemote() error {
	err := p.writeCmd([]byte{CmdClose, p.term, 0, 0})
	if err != nil {
		return err
	}
	log.Debugf("[%s] interrupt sent to remote", p)

	return nil
}

func (p *Pipe) TryPing() error {
	return p.writeCmd([]byte{CmdPing, p.term, 0, 0})
}

func (p *Pipe) writeCmd(b []byte) error {
	m := len(Magic)
	buf := make([]byte, m+len(b))
	copy(buf[:m], Magic)
	copy(buf[m:], b)
	_, err := p.conn.Write(buf)
	return err
}

func min(nums ...int) int {
	n := math.MaxInt64
	for _, num := range nums {
		if num < n {
			n = num
		}
	}
	return n
}

func max(nums ...int) int {
	n := math.MinInt64
	for _, num := range nums {
		if num > n {
			n = num
		}
	}
	return n
}

func (p *Pipe) IsPipeErr(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*net.OpError); ok {
		addr := p.conn.LocalAddr().String()
		return e != nil && (addr == e.Source.String() || addr == e.Addr.String())
	}
	return strings.ContainsAny(err.Error(), ErrPipe.Error())
}

func IsWriteError(err error) bool {
	if e, ok := err.(*net.OpError); ok {
		if e.Op != "write" {
			return false
		}
	}
	return errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}

func IsReadError(err error) bool {
	if e, ok := err.(*net.OpError); ok {
		return e.Op == "read"
	}

	return false
}

func IsTimeoutError(err error) bool {
	if e, ok := err.(net.Error); ok {
		return e.Timeout()
	}
	return false
}

func IsReadFinish(err error) bool {
	return err == nil || err == io.EOF
}

func IsInterrupted(err error) bool {
	if e, ok := err.(*net.OpError); ok {
		return e.Err.Error() == ErrInterrupted.Error()
	}
	return false
}
