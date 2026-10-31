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

package stats

import (
	"context"
	"io"
	"log"
	"net"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoopbackAddressForms(t *testing.T) {
	for _, tc := range []struct {
		addr string
		want bool
	}{
		{"malformed", false}, {":8080", false}, {"127.0.0.1:80", true}, {"[::1]:80", true}, {"192.0.2.1:80", false}, {"localhost:80", true}, {"invalid host:80", false},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			if got := isLoopbackHost(tc.addr); got != tc.want {
				t.Fatalf("isLoopbackHost=%v want %v", got, tc.want)
			}
		})
	}
}

func TestEndpointLifecycleAndBindFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s := &Server{Addr: "127.0.0.1:0", Logger: log.New(cancelOnLog{cancel}, "", 0)}
	if err := s.ListenAndServe(ctx); err != nil {
		t.Fatal(err)
	}
	// An already cancelled context must also shut down a successfully bound server.
	if err := (&Server{Addr: "127.0.0.1:0"}).ListenAndServe(ctx); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	if err := (&Server{Addr: ln.Addr().String()}).ListenAndServe(ctx); err == nil {
		t.Fatal("occupied listener accepted")
	}
	if err := (&Server{Addr: "invalid", AllowPublic: true}).ListenAndServe(ctx); err == nil {
		t.Fatal("invalid address accepted")
	}
	s.Logger = log.New(io.Discard, "", 0)
	s.logf("%s", "custom logger")
}

type cancelOnLog struct{ cancel context.CancelFunc }

func (w cancelOnLog) Write(p []byte) (int, error) { w.cancel(); return len(p), nil }

func TestUnknownEndpointIsNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	(&Server{}).Handler().ServeHTTP(w, httptest.NewRequest("GET", "/unknown", nil))
	if w.Code != 404 {
		t.Fatalf("unknown endpoint: %d", w.Code)
	}
}
