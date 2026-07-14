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

package i18n

import (
	"regexp"
	"testing"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
		ok   bool
	}{
		{"zh_CN.UTF-8", Chinese, true},
		{"zh-CN", Chinese, true},
		{"zh", Chinese, true},
		{"en_US.UTF-8", English, true},
		{"en", English, true},
		{"ja_JP", Japanese, true},
		{"ko-KR", Korean, true},
		{"fr_FR@euro", French, true},
		{"de-DE", German, true},
		{"C", English, false},
		{"POSIX", English, false},
		{"", English, false},
		{"pt_BR", English, false},
	}
	for _, c := range cases {
		got, ok := match(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("match(%q)=(%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDetectFromEnv(t *testing.T) {
	// 清空可能干扰的变量，仅设置目标变量。
	for _, k := range []string{"PORTMAP_LANG", "LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		t.Setenv(k, "")
	}

	t.Setenv("PORTMAP_LANG", "ja")
	if got := Detect(); got != Japanese {
		t.Errorf("PORTMAP_LANG=ja -> %q, want %q", got, Japanese)
	}

	t.Setenv("PORTMAP_LANG", "")
	t.Setenv("LANG", "fr_FR.UTF-8")
	if got := Detect(); got != French {
		t.Errorf("LANG=fr_FR.UTF-8 -> %q, want %q", got, French)
	}

	// LANGUAGE 优先级列表：取第一个可识别项。
	t.Setenv("LANG", "")
	t.Setenv("LANGUAGE", "xx:de:en")
	if got := Detect(); got != German {
		t.Errorf("LANGUAGE=xx:de:en -> %q, want %q", got, German)
	}
}

func TestTFallback(t *testing.T) {
	SetLang(German)
	// 已知 key 应返回德文。
	if got := T(KeyFlagVersion); got != messagesDE[KeyFlagVersion] {
		t.Errorf("T(KeyFlagVersion)=%q, want %q", got, messagesDE[KeyFlagVersion])
	}
	// 未知 key 应回退为 key 本身。
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("T(unknown)=%q, want key itself", got)
	}
	// 带参数会格式化。
	if got := T(KeyErrListenPort, 70000); got == messagesDE[KeyErrListenPort] {
		t.Errorf("T with args should format, got raw %q", got)
	}
	SetLang(English)
}

func TestAllLanguagesHaveAllKeys(t *testing.T) {
	for key := range messagesEN {
		for lang, table := range messages {
			if _, ok := table[key]; !ok {
				t.Errorf("language %q missing key %q", lang, key)
			}
		}
	}
}

// verbRe 匹配 fmt 占位符（含 %%、%w、%q、%d 等），用于校验各语言译文与
// 英文的占位符序列一致，避免运行时出现 %!x(...) 之类的格式化错误。
var verbRe = regexp.MustCompile(`%[+\-# 0-9.*]*[a-zA-Z%]`)

func placeholders(s string) []string {
	out := verbRe.FindAllString(s, -1)
	if out == nil {
		return []string{}
	}
	return out
}

func TestPlaceholdersConsistent(t *testing.T) {
	for key, enText := range messagesEN {
		want := placeholders(enText)
		for lang, table := range messages {
			if lang == English {
				continue
			}
			got := placeholders(table[key])
			if len(got) != len(want) {
				t.Errorf("language %q key %q: placeholder count %d != en %d (%v vs %v)",
					lang, key, len(got), len(want), got, want)
				continue
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("language %q key %q: placeholder[%d]=%q != en %q",
						lang, key, i, got[i], want[i])
				}
			}
		}
	}
}
