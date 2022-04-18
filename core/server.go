package core

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"

	socks5 "github.com/armon/go-socks5"
	"github.com/hashicorp/yamux"
)

var session *yamux.Session

type ReverseSocks5 struct {
	// 监听一个不需要别人知道的端口
	Address string
	// 所有端点使用相同的key
	Key string
}

func NewReverseSocks5(key string) *ReverseSocks5 {
	r := ReverseSocks5{}
	r.Address = ":" + strconv.Itoa(r.GetPort())
	r.Key = key

	return &r
}

// GetFreePort asks the kernel for a free open port that is ready to use.
func (r *ReverseSocks5) GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// GetPort is deprecated, use GetFreePort instead
// Ask the kernel for a free open port that is ready to use
func (r *ReverseSocks5) GetPort() int {
	port, err := r.GetFreePort()
	if err != nil {
		log.Println(err)
	}
	return port
}

// close log
func (r *ReverseSocks5) CloseLog() {
	log.SetOutput(ioutil.Discard)
}

func (r *ReverseSocks5) Sha1(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	return hex.EncodeToString(bs)
}

// reverse server，该代码通常在目标机器上运行，不能直连，但是能够向外连接的情况
// address 的目标连接
// 通常是内网中的目标运行该方法
// 或者不能从外部连接的目标，但是可以从内部连接外面，才用到该方法
func (r *ReverseSocks5) ConnectForSocks() error {
	cred := socks5.StaticCredentials{
		r.Sha1(r.Key): r.Key,
	}
	cator := socks5.UserPassAuthenticator{Credentials: cred}
	conf := &socks5.Config{AuthMethods: []socks5.Authenticator{cator}}
	// 不能用密码，因为另外一端并不知道密码是什么
	//conf := &socks5.Config{}
	server, err := socks5.New(conf)
	if err != nil {
		return err
	}

	var conn net.Conn
	log.Println("Connecting to far end")
	conn, err = net.Dial("tcp", r.Address)
	if err != nil {
		return err
	}

	log.Println("Starting server")
	session, err = yamux.Server(conn, nil)
	if err != nil {
		return err
	}

	for {
		stream, err := session.Accept()
		log.Println("Acceping stream")
		if err != nil {
			return err
		}
		log.Println("Passing off to socks5")
		go func() {
			err = server.ServeConn(stream)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}

// Catches yamux connecting to us
// 监听的这个地址和端口和ConnectForSocks 的地址要一致
// ListenForSocks 要先运行，并通过key关联通知 ConnectForSocks
func (r *ReverseSocks5) ListenForSocks() {
	log.Println("Listening for the far end")
	ln, err := net.Listen("tcp", r.Address)
	if err != nil {
		return
	}
	for {
		conn, err := ln.Accept()
		log.Println("Got a client")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Errors accepting!")
		}
		// Add connection to yamux
		session, err = yamux.Client(conn, nil)
	}
}

// Catches clients and connects to yamux
// 本地监听的代理端口，实际上是远程反弹过来的socks
func (r *ReverseSocks5) ListenForClients(address string) error {
	log.Println("Waiting for clients")
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		// TODO dial socks5 through yamux and connect to conn
		if session == nil {
			log.Println("The remote bounce connection is not here yet")
			conn.Close()
			continue
		}
		log.Println("Got a client\nOpening a stream")
		stream, err := session.Open()
		if err != nil {
			return err
		}
		// connect both of conn and stream
		go func() {
			log.Println("Starting to copy conn to stream")
			io.Copy(conn, stream)
			conn.Close()
		}()
		go func() {
			log.Println("Starting to copy stream to conn")
			io.Copy(stream, conn)
			stream.Close()
			log.Println("Done copying stream to conn")
		}()
	}
}
