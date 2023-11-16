package sockettype

import "github.com/obnahsgnaw/application/servertype"

// SocketType 服务类型
type SocketType string

func (s SocketType) String() string {
	return string(s)
}

func (s SocketType) ToServerType() servertype.ServerType {
	switch s {
	case TCP, TCP4, TCP6:
		return servertype.Tcp
	case WSS:
		return servertype.Wss
	case UDP, UDP4, UDP6:
		return servertype.Udp
	default:
		panic("trans socket type to server type failed")
	}
}

func (s SocketType) ToHandlerType() servertype.ServerType {
	switch s {
	case TCP, TCP4, TCP6:
		return servertype.TcpHdl
	case WSS:
		return servertype.WssHdl
	case UDP, UDP4, UDP6:
		return servertype.UdpHdl
	default:
		panic("trans socket type to server type failed")
	}
}

// 服务类型定义
const (
	TCP  SocketType = "tcp"
	TCP4 SocketType = "tcp4"
	TCP6 SocketType = "tcp6"
	UDP  SocketType = "udp"
	UDP4 SocketType = "udp4"
	UDP6 SocketType = "udp6"
	WSS  SocketType = "wss"
)
