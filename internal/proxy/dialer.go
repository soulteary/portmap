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

package proxy

import (
	"context"
	"net"
	"time"
)

// Dialer 描述建立出站连接的能力。抽象出来便于测试与替换。
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// directDialer 是忽略一切环境代理的直连拨号器。
//
// 它有意不使用 golang.org/x/net/proxy.FromEnvironment 或
// http.ProxyFromEnvironment，因此环境中的代理配置不会生效。
type directDialer struct {
	d *net.Dialer
}

// NewDirectDialer 创建一个直连拨号器，忽略所有环境代理设置。
func NewDirectDialer(timeout, keepAlive time.Duration) Dialer {
	return &directDialer{
		d: &net.Dialer{
			Timeout:   timeout,
			KeepAlive: keepAlive,
		},
	}
}

func (dd *directDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dd.d.DialContext(ctx, network, address)
}
