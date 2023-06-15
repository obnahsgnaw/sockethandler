package sockettype

// SocketType 服务类型
type SocketType string

func (s SocketType) String() string {
	return string(s)
}

// 服务类型定义
const (
	TCP SocketType = "tcp"
	UDP SocketType = "udp"
	WSS SocketType = "wss"
)
