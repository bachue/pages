package sshd

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/bachue/pages/config"
	"github.com/bachue/pages/log_driver"
	"golang.org/x/crypto/ssh"
)

type Server struct {
	Config       *config.SshdConfig
	ServerConfig *ssh.ServerConfig
	Logger       *log_driver.Logger
	ClientCount  int32
}

func NewServer(sshdConfig *config.SshdConfig, logger *log_driver.Logger) (*Server, error) {
	serverConfig, err := getSshServerConfig(sshdConfig)
	if err != nil {
		return nil, err
	}
	return &Server{Config: sshdConfig, ServerConfig: serverConfig, Logger: logger, ClientCount: 0}, nil
}

func (server *Server) Start() error {
	listener, err := server.doListen()
	if err != nil {
		return err
	}
	defer listener.Close()
	for {
		server.doAccept(listener)
	}
	return nil
}

func (server *Server) doListen() (net.Listener, error) {
	host_port := server.getHostPort()
	listener, err := net.Listen("tcp", host_port)
	if err != nil {
		server.Logger.Errorf("Failed to listen on %s due to %s", host_port, err)
		return nil, err
	}
	server.Logger.Infof("Listening on %s", host_port)
	return listener, nil
}

func (server *Server) doAccept(listener net.Listener) {
	tcpConn, err := listener.Accept()
	if err != nil {
		// TODO: Sleep to retry on accept failure, refer to: https://golang.org/src/net/http/server.go#L1883
		server.Logger.Errorf("Failed to accept connection due to %s", err)
		return
	}
	server.Logger.Debugf("Accepted incoming connection from %s", tcpConn.RemoteAddr().String())
	go server.handleConnection(tcpConn)
}

func (server *Server) handleConnection(conn net.Conn) {
	showConnCount := func() {
		server.Logger.Debugf("Current Connections: (%d/%d)",
			atomic.LoadInt32(&server.ClientCount), server.Config.MaxClient)
	}

	defer func() {
		conn.Close()
		server.Logger.Debugf("The connection from %s is closed", conn.RemoteAddr().String())
		atomic.AddInt32(&server.ClientCount, -1)
		showConnCount()
	}()

	atomic.AddInt32(&server.ClientCount, 1)
	showConnCount()
	if atomic.LoadInt32(&server.ClientCount) > server.Config.MaxClient {
		server.Logger.Errorf("Failed to accept incoming connection due to too many connections (%d/%d)",
			server.ClientCount, server.Config.MaxClient)
		return
	}

	sshConnection, chans, reqs, err := ssh.NewServerConn(conn, server.ServerConfig)
	if err != nil {
		if err != io.EOF {
			server.Logger.Errorf("Failed to start SSL connection with %s due to %s",
				conn.RemoteAddr().String(), err)
		}
		return
	}
	server.Logger.Debugf("Built SSL connection with %s, sessionId: %s, client version: %s, user: %s",
		conn.RemoteAddr().String(), hex.EncodeToString(sshConnection.SessionID()),
		sshConnection.ClientVersion(), sshConnection.User())
	go ssh.DiscardRequests(reqs)
	server.handleChannels(chans, sshConnection)
}

func (server *Server) handleChannels(chans <-chan ssh.NewChannel, conn *ssh.ServerConn) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		server.Logger.Debugf("New SSH Channel Request %s from %s", newChannel.ChannelType(), conn.RemoteAddr().String())
		// TODO: Find Channel ID to log
		go server.handleChannel(newChannel, conn)
	}
}

func (server *Server) handleChannel(newChannel ssh.NewChannel, conn *ssh.ServerConn) {
	channelType := newChannel.ChannelType()
	if channelType != "session" {
		newChannel.Reject(ssh.UnknownChannelType,
			fmt.Sprintf("Unknown SSH Channel Type: %s, only `session` is supported", channelType))
		server.Logger.Errorf("Rejected SSH Channel Request from %s due to unknown channel type: %s",
			conn.RemoteAddr().String(), newChannel.ChannelType())
		return
	}
	channel, requests, err := newChannel.Accept()
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, "Failed to accept SSH Channel Request, developers are working on it.")
		server.Logger.Errorf("Rejected SSH Channel Request from %s due to accept request failure: %s",
			conn.RemoteAddr().String(), err)
		return
	}
	server.Logger.Debugf("Accepted new SSH Channel Request from %s", conn.RemoteAddr().String())

	server.handleRequest(channel, requests, conn)
}

func (server *Server) handleRequest(channel ssh.Channel, requests <-chan *ssh.Request, conn *ssh.ServerConn) {
	defer func() {
		err := channel.Close()
		if err != nil {
			server.Logger.Errorf("Failed to close SSH Channel from %s due to %s",
				conn.RemoteAddr().String(), err)
		}
		server.Logger.Debugf("Close SSH Channel from %s", conn.RemoteAddr().String())
	}()
	for req := range requests {
		server.Logger.Debugf("Received new SSH Request (type = %s) from %s", req.Type, conn.RemoteAddr().String())

		switch req.Type {
		case "exec":
			server.handleExecRequest(channel, req, conn)
		default:
			var err error
			if req.Type == "env" {
				_, err = channel.Stderr().Write([]byte("error: Pages does not support SendEnv.\n"))
			} else {
				_, err = channel.Write([]byte("You've successfully authenticated, but Pages does not provide shell access.\n"))
			}
			if err != nil && err != io.EOF {
				server.Logger.Errorf("Failed to Talk to SSH Request due to %s", err)
			}
			err = req.Reply(false, nil)
			if err != nil && err != io.EOF {
				server.Logger.Errorf("Failed to Reply false to SSH Request due to %s", err)
			}
			err = channel.Close()
			if err != nil && err != io.EOF {
				server.Logger.Errorf("Failed to close SSH Request due to %s", err)
			}
			server.Logger.Errorf("Close SSH Request due to unsupported SSH Request type: %s", req.Type)
		}
		return
	}
}

func (server *Server) handleExecRequest(channel ssh.Channel, request *ssh.Request, conn *ssh.ServerConn) {
	doReply := func(ok bool) {
		err := request.Reply(ok, nil)
		if err != nil {
			server.Logger.Errorf("Failed to reply %t to SSH Request from %s due to %s",
				ok, conn.RemoteAddr().String(), err)
		}
		server.Logger.Debugf("Reply to SSH Request `%t` from %s", ok, conn.RemoteAddr().String())
	}
	if len(request.Payload) < 4 {
		server.Logger.Errorf("Payload must not be shorter than 4 bytes, but only %d bytes", len(request.Payload))
		doReply(false)
		return
	}
	header := request.Payload[:4]
	cmdLen := int64(binary.BigEndian.Uint32(header))
	if int64(len(request.Payload)) < 4+cmdLen {
		server.Logger.Errorf("Payload must not be shorter than %d bytes, but only %d bytes", 4+cmdLen, len(request.Payload))
		doReply(false)
		return
	}
	cmd := request.Payload[4 : 4+cmdLen]
	server.Logger.Debugf("Execute command `%s` via SSH from %s",
		string(cmd), conn.RemoteAddr().String())

	shellCmd := exec.Command(server.Config.ShellPath, "-c", string(cmd))

	stdinPipe, err := shellCmd.StdinPipe()
	if err != nil {
		server.Logger.Errorf("Failed to create STDIN pipe error for command: %s", err)
		doReply(false)
		return
	}
	defer stdinPipe.Close()

	stdoutPipe, err := shellCmd.StdoutPipe()
	if err != nil {
		server.Logger.Errorf("Failed to create STDOUT pipe error for command: %s", err)
		doReply(false)
		return
	}
	defer stdoutPipe.Close()

	stderrPipe, err := shellCmd.StderrPipe()
	if err != nil {
		server.Logger.Errorf("Failed to create STDERR pipe error for command: %s", err)
		doReply(false)
		return
	}
	defer stderrPipe.Close()

	sendExitStatus := func() {
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
		server.Logger.Debugf("Sent exit status 0 to %s", conn.RemoteAddr().String())
	}

	var once sync.Once

	go func() {
		io.Copy(stdinPipe, channel)
		once.Do(sendExitStatus)
	}()
	go func() {
		io.Copy(channel, stdoutPipe)
		once.Do(sendExitStatus)
	}()
	go func() {
		io.Copy(channel.Stderr(), stderrPipe)
		once.Do(sendExitStatus)
	}()

	err = shellCmd.Start()
	if err != nil {
		server.Logger.Errorf("Close SSH Channel from %s due to command error: %s", conn.RemoteAddr().String(), err)
		doReply(false)
		return
	}

	doReply(true)
	_, err = shellCmd.Process.Wait()
	if err != nil {
		_, ok := err.(*exec.ExitError)
		if !ok {
			server.Logger.Errorf("Failed to wait command(PID = %d) due to %s", shellCmd.Process.Pid, err)
		}
		return
	}
}

func (server *Server) getHostPort() string {
	host_port := server.Config.ListenHost
	host_port += ":"
	host_port += strconv.Itoa(int(server.Config.ListenPort))
	return host_port
}

func getSshServerConfig(sshdConfig *config.SshdConfig) (*ssh.ServerConfig, error) {
	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn

	serverConfig := ssh.ServerConfig{
		//Define a function to run when a client attempts a password login
		// TODO: Avoid Login by Password
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in a production setting.
			if c.User() == "foo" && string(pass) == "bar" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
		// TODO: Use PublicKeyCallback instead
		// PublicKeyCallback: func(conn ssh.ConnMetadata, key crypto.PublicKey) (*ssh.Permissions, error) {
		// TODO
		// }
		// TODO:
		// AuthLogCallback: func(conn ssh.ConnMetadata, method string, err error) {
		// TODO
		// }
		//
		// You may also explicitly allow anonymous client authentication, though anon bash
		// sessions may not be a wise idea
		// NoClientAuth: true,
	}
	private_key, err := getPrivateKey(sshdConfig)
	if err != nil {
		return nil, err
	}
	serverConfig.AddHostKey(private_key)
	return &serverConfig, nil
}

func getPrivateKey(sshdConfig *config.SshdConfig) (ssh.Signer, error) {
	private, err := ssh.ParsePrivateKey([]byte(sshdConfig.PrivateKey))
	return private, err
}
