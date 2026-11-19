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

package netutil

import (
	"bytes"
	"io"
	"net"
	"testing"
)

func TestProgressCopiesAndHalfCloses(t *testing.T) {
	a, b := net.Pipe()
	c, d := net.Pipe()
	defer func() { _ = a.Close(); _ = b.Close(); _ = c.Close(); _ = d.Close() }()
	up, down := make(chan int64, 1), make(chan int64, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		RelayReaderProgress(b, b, c, func(n int64) { up <- n }, func(n int64) { down <- n })
	}()
	go func() { _, _ = a.Write([]byte("up")) }()
	if _, err := io.ReadFull(d, make([]byte, 2)); err != nil {
		t.Fatal(err)
	}
	if n := <-up; n != 2 {
		t.Fatal(n)
	}
	go func() { _, _ = d.Write([]byte("down")) }()
	if _, err := io.ReadFull(a, make([]byte, 4)); err != nil {
		t.Fatal(err)
	}
	if n := <-down; n != 4 {
		t.Fatal(n)
	}
	_ = a.Close()
	_ = d.Close()
	<-done
	var buf bytes.Buffer
	w := progressWriter{w: &buf}
	if n, err := w.Write(nil); n != 0 || err != nil {
		t.Fatalf("%d %v", n, err)
	}
	if n, err := w.Write([]byte("x")); n != 1 || err != nil {
		t.Fatalf("%d %v", n, err)
	}
}

type partialProgressWriter struct{}

func (partialProgressWriter) Write([]byte) (int, error) { return 2, io.ErrClosedPipe }
func TestProgressCountsPartialWrite(t *testing.T) {
	var counted int64
	w := progressWriter{w: partialProgressWriter{}, add: func(n int64) { counted += n }}
	n, err := w.Write([]byte("hello"))
	if n != 2 || err != io.ErrClosedPipe || counted != 2 {
		t.Fatalf("n=%d err=%v counted=%d", n, err, counted)
	}
}
