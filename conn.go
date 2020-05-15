package main

import (
    "io"
    "net"
    "time"
    "bufio"
    "strconv"
)

type ConnHost struct {
    /* Hold host specific details */
    name     string
    hostport string
    fwdport  string
}

func (host *ConnHost) Name() string {
    return host.name
}

func (host *ConnHost) Port() string {
    return host.fwdport
}

func (host *ConnHost) RealPort() string {
    return host.hostport
}

type ConnClient struct {
    /* Hold client specific details */
    ip   string
    port string
}

func (client *ConnClient) Ip() string {
    return client.ip
}

func (client *ConnClient) Port() string {
    return client.port
}

func (client *ConnClient) AddrStr() string {
    return client.Ip()+":"+client.Port()
}

type GophorListener struct {
    /* Simple net.Listener wrapper that holds onto virtual
     * host information + generates Worker instances on Accept()
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

func (l *GophorListener) Accept() (*Worker, error) {
    conn, err := l.Listener.Accept()
    if err != nil {
        return nil, err
    }

    /* Should always be ok as listener is type TCP (see above) */
    addr, _ := conn.RemoteAddr().(*net.TCPAddr)
    client := &ConnClient{ addr.IP.String(), strconv.Itoa(addr.Port) }

    return &Worker{ NewBufferedDeadlineConn(conn), l.Host, client, l.Root }, nil
}

type DeadlineConn struct {
    /* Simple wrapper to net.Conn that sets deadlines
     * on each call to Read() / Write()
     */

    conn  net.Conn
}

func NewDeadlineConn(conn net.Conn) *DeadlineConn {
    return &DeadlineConn{ conn }
}

func (c *DeadlineConn) Read(b []byte) (int, error) {
    /* Implements a regular net.Conn + updates deadline */
    c.conn.SetReadDeadline(time.Now().Add(Config.SocketReadDeadline))
    return c.conn.Read(b)
}

func (c *DeadlineConn) Write(b []byte) (int, error) {
    /* Implements a regular net.Conn + updates deadline */
    c.conn.SetWriteDeadline(time.Now().Add(Config.SocketWriteDeadline))
    return c.conn.Write(b)
}

func (c *DeadlineConn) Close() error {
    /* Close */
    return c.conn.Close()
}

type BufferedDeadlineConn struct {
    /* Wrapper around DeadlineConn that provides buffered
     * reads and writes.
     */

    conn   *DeadlineConn
    buffer *bufio.ReadWriter
}

func NewBufferedDeadlineConn(conn net.Conn) *BufferedDeadlineConn {
    deadlineConn := NewDeadlineConn(conn)
    return &BufferedDeadlineConn{
        deadlineConn,
        bufio.NewReadWriter(
            bufio.NewReaderSize(deadlineConn, Config.SocketReadBufSize),
            bufio.NewWriterSize(deadlineConn, Config.SocketWriteBufSize),
        ),
    }
}

func (c *BufferedDeadlineConn) ReadLine() ([]byte, error) {
    /* Return slice */
    b := make([]byte, 0)

    for {
        /* Read line */
        line, isPrefix, err := c.buffer.ReadLine()
        if err != nil {
            return nil, err
        }

        /* Add to return slice */
        b = append(b, line...)

        /* If !isPrefix, we can break-out */
        if !isPrefix {
            break
        }
    }

    return b, nil
}

func (c *BufferedDeadlineConn) Write(b []byte) (int, error) {
    return c.buffer.Write(b)
}

func (c *BufferedDeadlineConn) WriteData(b []byte) error {
    _, err := c.buffer.Write(b)
    return err
}

func (c *BufferedDeadlineConn) WriteRaw(r io.Reader) error {
    _, err := c.buffer.ReadFrom(r)
    return err
}

func (c *BufferedDeadlineConn) Close() error {
    /* First flush buffer, then close */
    c.buffer.Flush()
    return c.conn.Close()
}
