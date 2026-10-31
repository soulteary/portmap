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

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestCLIExitCodes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want int
	}{{[]string{"-h"}, 0}, {[]string{"-unknown"}, 2}, {[]string{"-external", "invalid", "-conns", "1", "-duration", "250ms", "-warmup", "0", "-max-error-rate", "0"}, 1}, {[]string{"-external", "invalid", "-conns", "1", "-duration", "250ms", "-warmup", "0"}, 0}} {
		if got := runCLI(tc.args); got != tc.want {
			t.Fatalf("%v: exit=%d want=%d", tc.args, got, tc.want)
		}
	}
}
func TestRoundTripMismatchAndZeroElapsed(t *testing.T) {
	a, b := net.Pipe()
	defer func() { _ = a.Close(); _ = b.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1)
		_, _ = io.ReadFull(b, buf)
		_, _ = b.Write([]byte("z"))
	}()
	s := &stats{}
	if _, ok := tcpRoundTrip(context.Background(), a, []byte("x"), make([]byte, 1), s); ok {
		t.Fatal("mismatch accepted")
	}
	<-done
	if s.errMismatch.Load() != 1 {
		t.Fatal("mismatch not counted")
	}
	o := &options{maxSamples: 1}
	if err := printJSONReport(io.Discard, o, hostInfo{}, result{}, true, 0, false, true); err != nil {
		t.Fatal(err)
	}
	printReport(io.Discard, o, hostInfo{}, result{}, true, 0, false)
	if resultErrorRate(result{}) != 0 {
		t.Fatal("empty sample has errors")
	}
	var state uint64
	if nextSampleRandom(&state) == 0 {
		t.Fatal("zero RNG state stuck")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestJSONOutputFailure(t *testing.T) {
	o, err := parseFlags([]string{"-external", "invalid", "-conns", "1", "-duration", "250ms", "-warmup", "0", "-format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runWithOutput(o, failWriter{}, io.Discard); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}
func TestTCPStreamDialFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &stats{}
	w := &worker{o: &options{mode: "throughput", payload: 1}, addr: "invalid", s: s}
	w.runTCPStream(ctx, &net.Dialer{}, []byte("x"), make([]byte, 1), 1)
	if s.errDial.Load() != 1 {
		t.Fatal("dial failure not counted")
	}
	if operationDeadline(context.Background(), time.Second).IsZero() {
		t.Fatal("missing fallback deadline")
	}
}

func TestUDPRoundTripRejectsCorruptReply(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1)
		_, addr, err := server.ReadFromUDP(buf)
		if err == nil {
			_, _ = server.WriteToUDP([]byte("z"), addr)
		}
	}()
	client, err := net.DialUDP("udp", nil, server.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	s := &stats{}
	if _, ok := udpRoundTrip(context.Background(), client, []byte("x"), make([]byte, 1), s); ok {
		t.Fatal("UDP mismatch accepted")
	}
	<-done
	if s.errMismatch.Load() != 1 {
		t.Fatal("UDP mismatch not counted")
	}
}
