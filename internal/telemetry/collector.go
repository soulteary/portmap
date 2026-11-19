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

package telemetry

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// Listen never removes an existing Unix socket: it may belong to a live service.
// Unix listeners unlink their own socket on Close. TCP defaults are loopback-only.
func Listen(endpoint string) (net.Listener, error) { return ListenWithPublic(endpoint, false) }

// ListenWithPublic requires explicit opt-in for non-loopback TCP listeners.
func ListenWithPublic(endpoint string, allowPublic bool) (net.Listener, error) {
	network, address, err := Endpoint(endpoint)
	if err != nil {
		return nil, err
	}
	if network == "tcp" && !allowPublic {
		host, _, _ := net.SplitHostPort(address)
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("stats TCP listener requires a loopback IP")
		}
	}
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	if network == "unix" {
		if err := os.Chmod(address, 0600); err != nil {
			_ = ln.Close()
			return nil, err
		}
	}
	return ln, nil
}

// Serve bounds concurrent clients and closes idle or incompatible senders.
func Serve(ctx context.Context, ln net.Listener, store *Store) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	slots := make(chan struct{}, 128)
	stop := context.AfterFunc(ctx, func() { _ = ln.Close() })
	defer stop()
	defer func() { cancel(); wg.Wait() }()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case slots <- struct{}{}:
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-slots }()
				defer func() { _ = conn.Close() }()
				stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
				defer stop()
				_ = receive(conn, store)
			}()
		default:
			_ = conn.Close()
		}
	}
}

func receive(conn net.Conn, store *Store) error {
	if err := conn.SetDeadline(time.Now().Add(ioTimeout)); err != nil {
		return err
	}
	var h hello
	if err := readFrame(conn, &h); err != nil {
		return err
	}
	if h.Version != Version || len(h.BootID) != 32 {
		return fmt.Errorf("incompatible stats handshake")
	}
	if err := writeFrame(conn, ack{Version: Version}); err != nil {
		return err
	}
	for {
		if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return err
		}
		var b batch
		if err := readFrame(conn, &b); err != nil {
			return err
		}
		if b.BootID != h.BootID {
			return fmt.Errorf("stats process changed")
		}
		if err := store.accept(b); err != nil {
			return err
		}
		if err := writeFrame(conn, ack{Version, b.Seq}); err != nil {
			return err
		}
	}
}
