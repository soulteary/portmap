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

package forward

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
)

// udpBufferSize 是 UDP 读缓冲区大小，足以容纳最大 UDP 数据报。
const udpBufferSize = 64 * 1024

// udpBufPool 复用 relay 的读缓冲，降低高会话数下的内存分配与占用。
// 存放 *[]byte 以避免每次 Get/Put 产生额外分配。
var udpBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, udpBufferSize)
		return &b
	},
}

// serveUDP 处理 UDP 监听与转发。UDP 无连接，这里以客户端地址为键维护
// 一张会话表：每个客户端对应一条到目标的 UDP 连接，目标的回包按会话
// 转发回对应客户端。空闲超时（或默认 60s）后回收会话。
func (s *Server) serveUDP(ctx context.Context) error {
	lc := s.listenConfig()
	pc, err := lc.ListenPacket(ctx, "udp", s.cfg.Listen)
	if err != nil {
		return err
	}
	conn := pc.(*net.UDPConn)

	s.infof(i18n.T(i18n.KeyLogUDPListening),
		conn.LocalAddr(), s.cfg.Target, s.cfg.ReuseAddr, s.cfg.MaxConns, s.cfg.IdleTimeout)

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	targetAddr, err := net.ResolveUDPAddr("udp", s.cfg.Target)
	if err != nil {
		return err
	}

	idle := s.cfg.IdleTimeout
	if idle <= 0 {
		idle = 60 * time.Second
	}

	sessions := &udpSessions{
		table:  make(map[string]*udpSession),
		server: s,
		conn:   conn,
		target: targetAddr,
		idle:   idle,
	}

	buf := make([]byte, udpBufferSize)
	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				sessions.closeAll()
				s.wg.Wait()
				return nil
			}
			if isNormalClose(err) {
				continue
			}
			return err
		}
		sess, err := sessions.get(ctx, client)
		if err != nil {
			if errors.Is(err, errSessionLimit) {
				s.debugf(i18n.T(i18n.KeyLogUDPLimit), client)
			} else {
				s.warnf(i18n.T(i18n.KeyLogUDPDialFailed), s.cfg.Target, err)
			}
			continue
		}
		if _, err := sess.dst.Write(buf[:n]); err != nil {
			// 拿到的会话可能刚被 relay 关闭，只下线自己手里的这个会话
			// （仅当表中仍是本指针时删除），再重建后重试一次，避免误删/误关
			// 其他包刚重建或 relay 已回收的会话。
			sessions.removeIf(client, sess)
			sess.cancel()
			// 等待旧会话 relay 退出后再重建：relay 的 defer 才释放 sem 并
			// close(dst)，若此时直接 get() 会因 sem 尚未归还而被误判为限流。
			// 加超时兜底，避免 relay 异常卡住时主循环死等。
			select {
			case <-sess.done:
			case <-time.After(2 * time.Second):
			}
			sess, err = sessions.get(ctx, client)
			if err != nil {
				if errors.Is(err, errSessionLimit) {
					s.debugf(i18n.T(i18n.KeyLogUDPLimit), client)
				} else {
					s.warnf(i18n.T(i18n.KeyLogUDPDialFailed), s.cfg.Target, err)
				}
				continue
			}
			if _, err := sess.dst.Write(buf[:n]); err != nil {
				s.debugf(i18n.T(i18n.KeyLogUDPWriteTarget), err)
				continue
			}
		}
		sess.touch()
	}
}

// errSessionLimit 表示因 MaxConns 限流拒绝新建 UDP 会话。
var errSessionLimit = errors.New("udp session limit reached")

type udpSession struct {
	dst      *net.UDPConn
	client   *net.UDPAddr
	lastSeen atomic.Int64 // UnixNano
	connID   int64
	cancel   context.CancelFunc
	// done 在 relay 退出（sem 已归还、dst 已关闭）后被 close，
	// 供重建路径等待旧会话彻底退出，避免限流误拒。
	done chan struct{}
}

func (ss *udpSession) touch() { ss.lastSeen.Store(time.Now().UnixNano()) }

type udpSessions struct {
	mu     sync.Mutex
	table  map[string]*udpSession
	server *Server
	conn   *net.UDPConn
	target *net.UDPAddr
	idle   time.Duration
}

// get 返回（必要时新建）与 client 关联的会话。
// 新建会话时若已达 MaxConns 限流上限，返回 errSessionLimit。
func (m *udpSessions) get(ctx context.Context, client *net.UDPAddr) (*udpSession, error) {
	key := client.String()
	m.mu.Lock()
	if ss, ok := m.table[key]; ok {
		m.mu.Unlock()
		return ss, nil
	}
	m.mu.Unlock()

	// 并发限流：UDP 无排队语义，占满则拒绝新会话。
	if m.server.sem != nil {
		select {
		case m.server.sem <- struct{}{}:
		default:
			return nil, errSessionLimit
		}
	}

	dst, err := net.DialUDP("udp", nil, m.target)
	if err != nil {
		if m.server.sem != nil {
			<-m.server.sem
		}
		return nil, err
	}

	sctx, cancel := context.WithCancel(ctx)
	connID := m.server.total.Add(1)
	ss := &udpSession{dst: dst, client: client, connID: connID, cancel: cancel, done: make(chan struct{})}
	ss.touch()

	m.mu.Lock()
	// 双检查：并发下可能已被其他 goroutine 创建。
	if existing, ok := m.table[key]; ok {
		m.mu.Unlock()
		cancel()
		_ = dst.Close()
		if m.server.sem != nil {
			<-m.server.sem
		}
		return existing, nil
	}
	m.table[key] = ss
	active := m.server.active.Add(1)
	m.mu.Unlock()

	m.server.infof(i18n.T(i18n.KeyLogUDPSessionOpen), connID, client, m.target, active)

	m.server.wg.Add(1)
	go func() {
		defer m.server.wg.Done()
		m.relay(sctx, ss)
	}()
	return ss, nil
}

// relay 从目标读取回包并转发回客户端，同时基于空闲超时回收会话。
func (m *udpSessions) relay(ctx context.Context, ss *udpSession) {
	defer func() {
		// 仅当表中仍是自己这个会话指针时才删除，避免误删重建后的新会话。
		m.removeIf(ss.client, ss)
		m.server.active.Add(-1)
		if m.server.sem != nil {
			<-m.server.sem
		}
		_ = ss.dst.Close()
		close(ss.done)
		m.server.infof(i18n.T(i18n.KeyLogUDPSessionClose), ss.connID, ss.client)
	}()

	bufp := udpBufPool.Get().(*[]byte)
	defer udpBufPool.Put(bufp)
	buf := *bufp
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// 基于 lastSeen 动态计算下一次读取截止时间，消除最多 2*idle 的漂移。
		wait := m.idle - time.Since(time.Unix(0, ss.lastSeen.Load()))
		if wait <= 0 {
			return
		}
		_ = ss.dst.SetReadDeadline(time.Now().Add(wait))
		n, err := ss.dst.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				// 超时后重新计算：若仍未到空闲阈值则继续等待。
				continue
			}
			return
		}
		ss.touch()
		if _, err := m.conn.WriteToUDP(buf[:n], ss.client); err != nil {
			m.server.debugf(i18n.T(i18n.KeyLogUDPWriteClient), err)
			return
		}
	}
}

// removeIf 仅当表中 client 对应的仍是 ss 这个指针时，才将其从表中删除。
// 用于 relay 退出时安全下线，避免误删刚被重建的新会话。
func (m *udpSessions) removeIf(client *net.UDPAddr, ss *udpSession) {
	key := client.String()
	m.mu.Lock()
	if cur, ok := m.table[key]; ok && cur == ss {
		delete(m.table, key)
	}
	m.mu.Unlock()
}

// closeAll 关闭所有会话。
func (m *udpSessions) closeAll() {
	m.mu.Lock()
	sessions := make([]*udpSession, 0, len(m.table))
	for _, ss := range m.table {
		sessions = append(sessions, ss)
	}
	m.table = make(map[string]*udpSession)
	m.mu.Unlock()
	for _, ss := range sessions {
		ss.cancel()
		_ = ss.dst.Close()
	}
}
