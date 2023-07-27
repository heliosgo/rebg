package server

type SubServer interface {
	Run()
	// JoinConn(net.Conn, net.Conn)
}
