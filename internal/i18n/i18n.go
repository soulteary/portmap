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

// Package i18n 提供轻量级的多语言支持：自动检测系统语言，并按语言返回
// 面向用户的字符串（帮助文本、日志、错误消息）。支持中、英、日、韩、法、德，
// 无法识别时回退英文。可用环境变量 PORTMAP_LANG 或命令行 -lang 手动覆盖。
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Lang 表示一种受支持的界面语言（BCP-47 主标签）。
type Lang string

const (
	// English 英文（默认回退语言）。
	English Lang = "en"
	// Chinese 简体中文。
	Chinese Lang = "zh"
	// Japanese 日文。
	Japanese Lang = "ja"
	// Korean 韩文。
	Korean Lang = "ko"
	// French 法文。
	French Lang = "fr"
	// German 德文。
	German Lang = "de"
)

// order 决定 Codes 的返回顺序（也是 --help 中展示顺序）。
var order = []Lang{English, Chinese, Japanese, Korean, French, German}

var (
	mu       sync.RWMutex
	current  Lang
	detected bool
)

// Codes 返回全部受支持语言代码，用于 --help 展示与提示。
func Codes() []string {
	out := make([]string, 0, len(order))
	for _, l := range order {
		out = append(out, string(l))
	}
	return out
}

// SetLang 强制设置当前语言，主要供命令行 -lang 覆盖或测试使用。
func SetLang(l Lang) {
	mu.Lock()
	current = l
	detected = true
	mu.Unlock()
}

// Current 返回当前生效的语言（必要时先按系统环境自动检测）。
func Current() Lang {
	mu.RLock()
	if detected {
		defer mu.RUnlock()
		return current
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if !detected {
		current = Detect()
		detected = true
	}
	return current
}

// ParseLang 将任意语言字符串解析为受支持的 Lang。
// 支持形如 "zh"、"zh-CN"、"zh_CN.UTF-8"、"en-US" 的写法。
// 无法识别时返回 (English, false)。
func ParseLang(s string) (Lang, bool) {
	return match(s)
}

// Detect 依据环境变量与系统区域设置推断界面语言。
// 优先级：PORTMAP_LANG > LC_ALL > LC_MESSAGES > LANG > LANGUAGE > 系统区域（平台相关）。
func Detect() Lang {
	for _, key := range []string{"PORTMAP_LANG", "LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			// LANGUAGE 可能是 "de:en" 这样的优先级列表，取第一个可识别项。
			for _, item := range strings.Split(v, ":") {
				if l, ok := match(item); ok {
					return l
				}
			}
		}
	}
	if v := systemLocale(); v != "" {
		if l, ok := match(v); ok {
			return l
		}
	}
	return English
}

// match 从形如 "zh_CN.UTF-8"、"en-US"、"ja" 的字符串中解析出受支持语言。
func match(s string) (Lang, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "c" || s == "posix" {
		return English, false
	}
	// 去掉编码/修饰后缀：zh_CN.UTF-8 -> zh_cn，en-US -> en-us。
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "-", "_")
	primary := s
	if i := strings.Index(s, "_"); i >= 0 {
		primary = s[:i]
	}
	switch primary {
	case "zh":
		return Chinese, true
	case "en":
		return English, true
	case "ja":
		return Japanese, true
	case "ko":
		return Korean, true
	case "fr":
		return French, true
	case "de":
		return German, true
	}
	return English, false
}

// T 返回 key 对应当前语言的文本。若提供 args，则按 fmt 规则格式化后返回；
// 否则返回原始文本（含 %-占位符），适合作为 fmt.Errorf/log.Printf 的格式串。
// 当前语言缺失该 key 时回退英文；英文也缺失时返回 key 本身。
func T(key string, args ...any) string {
	format := lookup(Current(), key)
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// lookup 查找指定语言下 key 的文本，缺失时回退英文，再缺失返回 key。
func lookup(lang Lang, key string) string {
	if table, ok := messages[lang]; ok {
		if s, ok := table[key]; ok {
			return s
		}
	}
	if s, ok := messages[English][key]; ok {
		return s
	}
	return key
}
