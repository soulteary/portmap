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

import "testing"

func TestConnectionAccessors(t *testing.T) {
	c := New()
	c.ConnOpened()
	if c.ActiveConns() != 1 || c.TotalConns() != 1 {
		t.Fatal(c.Snapshot())
	}
	c.ConnClosed()
	if c.ActiveConns() != 0 || c.TotalConns() != 1 {
		t.Fatal(c.Snapshot())
	}
}
