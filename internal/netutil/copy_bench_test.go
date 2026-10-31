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
	"io"
	"net"
	"testing"
)

// Isolate generic relay-buffer allocation from socket and scheduler costs.
// Neither endpoint exposes WriterTo/ReaderFrom, as with timeout wrappers.
type benchmarkCopyConn struct{ net.Conn }

func (benchmarkCopyConn) Write(p []byte) (int, error) { return len(p), nil }
func (benchmarkCopyConn) CloseWrite() error           { return nil }

type benchmarkCopyReader struct{ remaining int }

func (r *benchmarkCopyReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	r.remaining -= n
	return n, nil
}
func BenchmarkCopyAndCloseWriteGeneric(b *testing.B) {
	var src benchmarkCopyReader
	dst := benchmarkCopyConn{}
	b.ReportAllocs()
	for b.Loop() {
		src.remaining = 64 * 1024
		n, err := CopyAndCloseWrite(dst, &src)
		if err != nil || n != 64*1024 {
			b.Fatalf("copy=%d %v", n, err)
		}
	}
}
