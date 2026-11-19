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

// Package telemetry transports cumulative statistics outside the forwarding path.
package telemetry

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"path/filepath"
	"time"

	"github.com/soulteary/portmap/internal/stats"
)

const Version = 1
const maxFrame = 4 << 20
const ioTimeout = 3 * time.Second

// Endpoint accepts an absolute Unix socket path or a TCP host and port.
func Endpoint(raw string) (network, address string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("invalid stats endpoint")
	}
	switch u.Scheme {
	case "unix":
		if u.Host == "" && filepath.IsAbs(u.Path) {
			return "unix", u.Path, nil
		}
	case "tcp":
		if u.Path == "" && u.Host != "" {
			host, port, e := net.SplitHostPort(u.Host)
			if e == nil && host != "" && port != "" {
				return "tcp", u.Host, nil
			}
		}
	}
	return "", "", fmt.Errorf("stats endpoint must be unix:///absolute/path or tcp://host:port")
}

// Sample identifies an instance within one process lifetime.
type Sample struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	stats.Snapshot
}

type batch struct {
	Version       int           `json:"version"`
	BootID        string        `json:"boot_id"`
	Seq           uint64        `json:"seq"`
	SampledAt     time.Time     `json:"sampled_at"`
	Samples       []Sample      `json:"samples"`
	Events        []stats.Event `json:"events,omitempty"`
	EventsCursor  uint64        `json:"events_cursor"`
	EventsDropped uint64        `json:"events_dropped"`
}

type hello struct {
	Version int    `json:"version"`
	BootID  string `json:"boot_id"`
}

type ack struct {
	Version int    `json:"version"`
	Seq     uint64 `json:"seq"`
}

func writeFrame(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(data) > maxFrame {
		return fmt.Errorf("stats frame exceeds limit")
	}
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(data)))
	if _, err = io.Copy(w, bytes.NewReader(size[:])); err != nil {
		return err
	}
	_, err = io.Copy(w, bytes.NewReader(data))
	return err
}

func readFrame(r io.Reader, v any) error {
	var size [4]byte
	if _, err := io.ReadFull(r, size[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(size[:])
	if n == 0 || n > maxFrame {
		return fmt.Errorf("invalid stats frame size")
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
