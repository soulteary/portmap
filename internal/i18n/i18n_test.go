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

// TestCodes 验证 Codes 返回全部受支持语言，且顺序与 order 一致。
func TestCodes(t *testing.T) {
	codes := Codes()
	if len(codes) != len(order) {
		t.Fatalf("Codes 返回 %d 项，期望 %d", len(codes), len(order))
	}
	for i, l := range order {
		if codes[i] != string(l) {
			t.Errorf("Codes[%d]=%q，期望 %q", i, codes[i], string(l))
		}
	}
	// 必须包含全部六种语言。
	want := map[string]bool{"en": true, "zh": true, "ja": true, "ko": true, "fr": true, "de": true}
	for _, c := range codes {
		delete(want, c)
	}
	if len(want) != 0 {
		t.Errorf("Codes 缺失语言: %v", want)
	}
}

// TestParseLang 验证公开的 ParseLang 与内部 match 行为一致。
func TestParseLang(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
		ok   bool
	}{
		{"zh-CN", Chinese, true},
		{"en_US.UTF-8", English, true},
		{"de", German, true},
		{"xx", English, false},
		{"", English, false},
	}
	for _, c := range cases {
		got, ok := ParseLang(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseLang(%q)=(%q,%v)，期望 (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestSetLangAndCurrent 验证 SetLang 覆盖后 Current 立即返回该语言。
func TestSetLangAndCurrent(t *testing.T) {
	orig := Current()
	t.Cleanup(func() { SetLang(orig) })

	SetLang(Japanese)
	if got := Current(); got != Japanese {
		t.Fatalf("SetLang(Japanese) 后 Current()=%q", got)
	}
	SetLang(French)
	if got := Current(); got != French {
		t.Fatalf("SetLang(French) 后 Current()=%q", got)
	}
}

// TestCurrentAutoDetect 验证未显式设置语言时 Current 走自动检测路径。
func TestCurrentAutoDetect(t *testing.T) {
	// 保存并在结束后恢复包级检测状态，避免影响其它测试。
	mu.Lock()
	savedCurrent, savedDetected := current, detected
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		current, detected = savedCurrent, savedDetected
		mu.Unlock()
	})

	for _, k := range []string{"PORTMAP_LANG", "LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "ko_KR.UTF-8")

	// 重置检测标志，强制 Current 重新检测。
	mu.Lock()
	detected = false
	mu.Unlock()

	if got := Current(); got != Korean {
		t.Fatalf("自动检测 Current()=%q，期望 ko", got)
	}
}

// TestDetectFallsBackToEnglish 验证所有来源都无法识别时回退英文。
func TestDetectFallsBackToEnglish(t *testing.T) {
	for _, k := range []string{"PORTMAP_LANG", "LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "xx_YY.UTF-8")
	if got := Detect(); got != English {
		t.Fatalf("无法识别时 Detect()=%q，期望 en", got)
	}
}

// TestDetectPriority 验证 PORTMAP_LANG 优先级高于其它环境变量。
func TestDetectPriority(t *testing.T) {
	for _, k := range []string{"PORTMAP_LANG", "LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "fr_FR.UTF-8")
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("PORTMAP_LANG", "zh")
	if got := Detect(); got != Chinese {
		t.Fatalf("PORTMAP_LANG 应优先，Detect()=%q，期望 zh", got)
	}
}

// TestLookupFallback 验证 lookup 在语言缺 key 时回退英文，英文也缺时返回 key。
func TestLookupFallback(t *testing.T) {
	if got := lookup(German, "no.such.key"); got != "no.such.key" {
		t.Errorf("缺失 key 应返回 key 本身，实际 %q", got)
	}
	// 已知 key 在德文表中存在时应返回德文文本。
	if got := lookup(German, KeyFlagVersion); got != messagesDE[KeyFlagVersion] {
		t.Errorf("lookup(German, KeyFlagVersion)=%q，期望 %q", got, messagesDE[KeyFlagVersion])
	}
	// 未知语言（无对应表）应回退英文表。
	if got := lookup(Lang("xx"), KeyFlagVersion); got != messagesEN[KeyFlagVersion] {
		t.Errorf("未知语言应回退英文，实际 %q", got)
	}
}

// TestTWithoutArgsReturnsRawFormat 验证不带参数时 T 返回原始格式串（含占位符）。
func TestTWithoutArgsReturnsRawFormat(t *testing.T) {
	SetLang(English)
	t.Cleanup(func() { SetLang(English) })
	raw := T(KeyErrListenPort)
	if raw != messagesEN[KeyErrListenPort] {
		t.Fatalf("无参数 T 应返回原始格式串，实际 %q", raw)
	}
}
