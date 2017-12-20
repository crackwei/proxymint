package main

import (
	"os"
	"time"
	"net"
	"syscall"
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

// LRW reads lines from a net.Conn with a specified timeout.
type LRW struct {
	conn    net.Conn
	scanner *bufio.Scanner
}

func NewLRW(conn net.Conn) *LRW {
	return &LRW{
		conn:    conn,
		scanner: bufio.NewScanner(conn),
	}
}

func (lrw *LRW) ReadStratumTimed(deadline time.Time) (Request, error) {
	if err := lrw.conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}

	if !lrw.scanner.Scan() {
		// Scanner will set error to nil on EOF.
		if lrw.scanner.Err() == nil {
			return nil, io.EOF
		}

		return nil, lrw.scanner.Err()
	}

	val, err := stratum.Parse(lrw.scanner.Bytes())
	if err != nil {
		return nil, err
	}

	return val, nil
}

// WaitForType waits until the client sends the expected message.
func (lrw *LRW) WaitForType(tp RequestType, deadline time.Time) (Request, error) {
	req, err := lrw.ReadStratumTimed(deadline)
	if err != nil {
		return nil, err
	}

	if req.Type() != tp {
		return nil, errors.New("unexpected message")
	}

	return req, nil
}

func (lrw *LRW) WriteStratumTimed(resp Response, deadline time.Time) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	data = append(data, byte('\n'))

	if err := lrw.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}

	if _, err := lrw.conn.Write(data); err != nil {
		return err
	}

	return nil
}

func (lrw *LRW) WriteStratumRaw(data []byte, deadline time.Time) error {
	if err := lrw.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}

	_, err := lrw.conn.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// Sets TCP_KEEPIDLE(= 120), TCP_KEEPINTVL(= 1) and TCP_KEEPCNT(= 5) if avaliable
func configureKeepAlive(c net.Conn, idleTime time.Duration, count int, interval time.Duration) error {
	conn, ok := c.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("Bad connection type: %T", c)
	}
	if err := conn.SetKeepAlive(true); err != nil {
		return err

	var f *os.File
	if f, err = conn.File(); err != nil {
		return err
	}
	defer f.Close()

	fd := int(f.Fd())
	d += (time.Second - time.Nanosecond)
	idle := int(idleTime.Seconds())
	intl := int(interval.Seconds())

	if err = os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPALIVE, idle)); err != nil {
		return err
	}

	// only work on UNIX systems
	if err = os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_TCP_KEEPINTVL, count)); err != nil {
		return err
	}
	if err = os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_TCP_KEEPINTVL, intl)); err != nil {
		return err
	}
	return nil
}

// Get the IP of the address, return TCPAddr.IP
// TODO: use inet_ntop
// func serialiseAddr(sa syscall.Sockaddr) []byte { 
// 	switch sa := sa.(type) {
// 	case *syscall.SockaddrInet4:
// 		return sa.Addr[0:]
// 	case *syscall.SockaddrInet6:
// 		return sa.Addr[0:]
// 	}
// 	return nil
// }

// Modified version of 'connect' with a timeout
// refer to https://blog.booking.com/socket-timeout-made-easy.html
// func connectTimeouts(sock socket, addr Sockaddr, timeout int) {
// }