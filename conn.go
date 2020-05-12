package main

import (
    "net"
    "time"
    "strconv"
)

type ConnHost struct {
    /* Hold host specific details */
    HostName string
    HostPort string
    FwdPort  string
}

func (host *ConnHost) Name() string {
    return host.HostName
}

func (host *ConnHost) Port() string {
    return host.FwdPort
}

func (host *ConnHost) RealPort() string {
    return host.HostPort
}

func (host *ConnHost) AddrStr() string {
    return host.Name()+":"+host.Port()
}

type ConnClient struct {
    /* Hold client specific details */
    ClientIp   string
    ClientPort string
}

func (client *ConnClient) Ip() string {
    return client.ClientIp
}

func (client *ConnClient) Port() string {
    return client.ClientPort
}

func (client *ConnClient) AddrStr() string {
    return client.Ip()+":"+client.Port()
}

type GophorListener struct {
    /* Simple net.Listener wrapper that holds onto virtual
     * host information + generates GophorConn instances
     */

    Listener net.Listener
    Host     *ConnHost
    Root     string
}

func BeginGophorListen(bindAddr, hostname, port, fwdPort, rootDir string) (*GophorListener, error) {
    gophorListener := new(GophorListener)
    gophorListener.Host = &ConnHost{ hostname, port, fwdPort }
    gophorListener.Root = rootDir

    var err error
    gophorListener.Listener, err = net.Listen("tcp", bindAddr+":"+port)
    if err != nil {
        return nil, err
    } else {
        return gophorListener, nil
    }
}

func (l *GophorListener) Accept() (*GophorConn, error) {
    conn, err := l.Listener.Accept()
    if err != nil {
        return nil, err
    }

    gophorConn := new(GophorConn)
    gophorConn.Conn = &DeadlineConn{ conn }

    /* Copy over listener host */
    gophorConn.Host = l.Host
    gophorConn.Root = l.Root

    /* Should always be ok as listener is type TCP (see above) */
    addr, _ := conn.RemoteAddr().(*net.TCPAddr)
    gophorConn.Client = &ConnClient{
        addr.IP.String(),
        strconv.Itoa(addr.Port),
    }

    return gophorConn, nil
}

type DeadlineConn struct {
    /* Simple wrapper to net.Conn that's sets deadlines
     * on each call to Read() / Write()
     */
    conn net.Conn
}

func (c *DeadlineConn) Read(b []byte) (int, error) {
    /* Implements reader + updates deadline */
    c.conn.SetReadDeadline(time.Now().Add(Config.SocketReadDeadline))
    return c.conn.Read(b)
}

func (c *DeadlineConn) Write(b []byte) (int, error) {
    /* Implements writer + updates deadline */
    c.conn.SetWriteDeadline(time.Now().Add(Config.SocketWriteDeadline))
    return c.conn.Write(b)
}

func (c *DeadlineConn) Close() error {
    /* Implements closer */
    return c.conn.Close()
}

type GophorConn struct {
    /* Wrap DeadlineConn with other connection details */

    Conn    *DeadlineConn
    Host    *ConnHost
    Client  *ConnClient
    Root    string
}

func (c *GophorConn) Read(b []byte) (int, error) {
    return c.Conn.Read(b)
}

func (c *GophorConn) Write(b []byte) (int, error) {
    return c.Conn.Write(b)
}

func (c *GophorConn) Close() error {
    return c.Conn.Close()
}

func (c *GophorConn) RootDir() string {
    return c.Root
}
