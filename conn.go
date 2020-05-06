package main

import (
    "net"
    "time"
    "strconv"
)

const (
    SocketReadTimeout  = time.Second
    SocketWriteTimeout = time.Second
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

func (l *GophorListener) Accept() (*GophorConnWrapper, error) {
    conn, err := l.Listener.Accept()
    if err != nil {
        return nil, err
    }

    connWrapper := new(GophorConnWrapper)
    connWrapper.Conn = &GophorConn{ conn }

    /* Copy over listener host */
    connWrapper.Host = l.Host
    connWrapper.Root = l.Root

    /* Should always be ok as listener is type TCP (see above) */
    addr, _ := conn.RemoteAddr().(*net.TCPAddr)
    connWrapper.Client = &ConnClient{
        addr.IP.String(),
        strconv.Itoa(addr.Port),
    }

    return connWrapper, nil
}

type GophorConn struct {
    conn net.Conn
}

func (c *GophorConn) Read(b []byte) (int, error) {
    /* Implements reader + updates deadline */
    c.conn.SetReadDeadline(time.Now().Add(SocketReadTimeout))
    return c.conn.Read(b)
}

func (c *GophorConn) Write(b []byte) (int, error) {
    /* Implements writer + updates deadline */
    c.conn.SetWriteDeadline(time.Now().Add(SocketWriteTimeout))
    return c.conn.Write(b)
}

func (c *GophorConn) Close() error {
    /* Implements closer */

    return c.conn.Close()
}

type GophorConnWrapper struct {
    /* Simple net.Conn wrapper with virtual host and client info */

    Conn    *GophorConn
    Host    *ConnHost
    Client  *ConnClient
    Root    string
}


func (c *GophorConnWrapper) RootDir() string {
    return c.Root
}
