// Copyright 2026 soulteary
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package netutil 汇聚 forward 与 proxy 共用的底层网络工具：
// 双向数据转发（relay）、单向拷贝 + 半关闭、以及带空闲超时的连接包装。
// 抽取到独立包避免 forward.pipe/idleConn 与 proxy.relay 的重复实现。
package netutil

import (
	"io"
	"net"
	"sync"
	"time"
)

// halfCloser 表示支持半关闭写方向（CloseWrite）的连接，TCP 连接均实现此接口。
type halfCloser interface {
	CloseWrite() error
}

// closeWrite 尝试半关闭 c 的写方向，使对端感知 EOF；
// 若连接不支持 CloseWrite，则以设置立即读截止时间的方式促使对端读取解除阻塞。
func closeWrite(c net.Conn) {
	if hc, ok := c.(halfCloser); ok {
		_ = hc.CloseWrite()
		return
	}
	_ = c.SetReadDeadline(time.Now())
}

// CopyAndCloseWrite 将 src 的数据拷贝到 dst，返回拷贝的字节数与拷贝错误。
// 拷贝结束后尝试半关闭 dst 的写方向，以便对端感知 EOF。
// 调用方可通过返回的 error 自行决定是否记录（例如过滤正常关闭类错误）。
func CopyAndCloseWrite(dst net.Conn, src io.Reader) (int64, error) {
	n, err := io.Copy(dst, src)
	closeWrite(dst)
	return n, err
}

// RelayReader 在 client 与 remote 之间双向拷贝数据，直到任意一方关闭。
//
// client 方向的读取来自 clientReader（通常是包裹了 client 的 bufio.Reader，
// 内部可能已缓冲了协议探测阶段读取的数据），写回客户端仍使用 client 连接本身。
// 若无需自定义读取源，clientReader 传入 client 即可。
//
// 每个方向拷贝完成后会尝试半关闭对端的写方向，以正确传播 EOF，
// 避免一端读到 EOF 后另一端仍然阻塞。
func RelayReader(client net.Conn, clientReader io.Reader, remote net.Conn) {
	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(remote, clientReader)
		closeWrite(remote)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, remote)
		closeWrite(client)
		done <- struct{}{}
	}()

	<-done
	<-done
}

// Relay 在 a 与 b 之间双向拷贝数据，是 RelayReader 在无缓冲读取源时的简写。
func Relay(a, b net.Conn) {
	RelayReader(a, a, b)
}

// idleGroup 为双向隧道共享活动状态。任一端发生 I/O 时同时刷新两端的全部
// 截止时间，使超时表示整条隧道无活动，而不是某个方向暂时静默。
type idleGroup struct {
	mu      sync.Mutex
	timeout time.Duration
	conns   []net.Conn
}

func (g *idleGroup) refreshDeadline() error {
	if g.timeout <= 0 {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	deadline := time.Now().Add(g.timeout)
	var firstErr error
	for _, conn := range g.conns {
		if err := conn.SetDeadline(deadline); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// IdleConn 包装 net.Conn。默认情况下 Read/Write 仅刷新各自方向的截止时间，
// 以免影响把它仅用作 reader 的调用方；ShareIdleTimeout 可把两个端点关联为
// 共享双向活动状态。
type IdleConn struct {
	net.Conn
	// Timeout 是允许的最长空闲时间；<=0 时不启用（等价于裸连接）。
	Timeout time.Duration
	group   *idleGroup
}

// ShareIdleTimeout 把隧道两端关联起来，使任一端的成功 I/O 都刷新两端的
// 读写截止时间。调用方应在开始并发中继前完成关联。
func ShareIdleTimeout(a, b *IdleConn, timeout time.Duration) {
	group := &idleGroup{
		timeout: timeout,
		conns:   []net.Conn{a.Conn, b.Conn},
	}
	a.Timeout, b.Timeout = timeout, timeout
	a.group, b.group = group, group
}

func (c *IdleConn) refreshReadDeadline() error {
	if c.group != nil {
		return c.group.refreshDeadline()
	}
	if c.Timeout > 0 {
		return c.SetReadDeadline(time.Now().Add(c.Timeout))
	}
	return nil
}

func (c *IdleConn) refreshWriteDeadline() error {
	if c.group != nil {
		return c.group.refreshDeadline()
	}
	if c.Timeout > 0 {
		return c.SetWriteDeadline(time.Now().Add(c.Timeout))
	}
	return nil
}

func (c *IdleConn) refreshSharedDeadlineAfterActivity(err error) error {
	if c.group == nil {
		return err
	}
	if refreshErr := c.group.refreshDeadline(); err == nil && refreshErr != nil {
		return refreshErr
	}
	return err
}

// Read 在读取前刷新读截止时间；共享模式还会在成功读取后刷新隧道两端。
func (c *IdleConn) Read(p []byte) (int, error) {
	if err := c.refreshReadDeadline(); err != nil {
		return 0, err
	}
	n, err := c.Conn.Read(p)
	if n > 0 {
		err = c.refreshSharedDeadlineAfterActivity(err)
	}
	return n, err
}

// Write 在写入前刷新写截止时间；共享模式还会在成功写入后刷新隧道两端。
func (c *IdleConn) Write(p []byte) (int, error) {
	if err := c.refreshWriteDeadline(); err != nil {
		return 0, err
	}
	n, err := c.Conn.Write(p)
	if n > 0 {
		err = c.refreshSharedDeadlineAfterActivity(err)
	}
	return n, err
}

// CloseWrite 保留底层 TCP 连接的半关闭能力，供 Relay 正确传播 EOF。
func (c *IdleConn) CloseWrite() error {
	if hc, ok := c.Conn.(halfCloser); ok {
		return hc.CloseWrite()
	}
	return c.SetReadDeadline(time.Now())
}
