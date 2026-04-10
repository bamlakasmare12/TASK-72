package pinyin

import (
	"strings"
	"unicode"
)

// pinyinTable maps common Chinese characters to their pinyin representations.
// Each rune key MUST appear exactly once. Characters with the same pinyin
// reading are distinct entries (different Unicode codepoints).
var pinyinTable = map[rune]string{
	// Engineering / software
	'工': "gong", '程': "cheng", '软': "ruan", '件': "jian",
	'开': "kai", '发': "fa", '数': "shu", '据': "ju",
	'科': "ke", '学': "xue", '金': "jin", '融': "rong",
	'云': "yun", '计': "ji", '算': "suan", '技': "ji2",
	'术': "shu2", '教': "jiao", '商': "shang", '业': "ye",
	'能': "neng", '合': "he", '规': "gui", '培': "pei",
	'训': "xun", '分': "fen", '析': "xi", '领': "ling",
	'导': "dao", '力': "li", '管': "guan", '理': "li2",
	'项': "xiang", '目': "mu", '财': "cai", '务': "wu",
	'报': "bao", '告': "gao", '安': "an", '全': "quan",
	'网': "wang", '络': "luo", '编': "bian", '码': "ma",
	'测': "ce", '试': "shi", '运': "yun2", '维': "wei",
	'设': "she", '部': "bu2", '署': "shu3", '人': "ren",
	'智': "zhi", '机': "ji3", '器': "qi", '深': "shen",
	'度': "du", '习': "xi2", '自': "zi",
	'然': "ran", '语': "yu", '言': "yan", '处': "chu",
	// Common
	'大': "da", '中': "zhong", '小': "xiao", '新': "xin",
	'旧': "jiu", '好': "hao", '高': "gao2", '低': "di",
	'上': "shang2", '下': "xia", '前': "qian", '后': "hou",
	'左': "zuo", '右': "you", '内': "nei", '外': "wai",
	'入': "ru", '出': "chu2", '来': "lai", '去': "qu",
	'做': "zuo2", '用': "yong", '有': "you2", '没': "mei",
	'是': "shi2", '不': "bu", '的': "de", '了': "le",
	'和': "he2", '在': "zai", '这': "zhe", '那': "na",
	'我': "wo", '你': "ni", '他': "ta", '她': "ta2",
	'它': "ta3", '们': "men", '最': "zui", '本': "ben",
	'年': "nian", '月': "yue", '日': "ri", '时': "shi3",
	'系': "xi3", '统': "tong", '方': "fang", '法': "fa2",
	'平': "ping", '台': "tai", '服': "fu", // '器' already above
	'框': "kuang", '架': "jia", /* '数' already above */ '库': "ku",
	'接': "jie", '口': "kou", '配': "pei2", '置': "zhi2",
	'文': "wen", '档': "dang", '资': "zi2", '源': "yuan",
	'基': "ji4", '础': "chu3", '进': "jin2", '阶': "jie2",
	'初': "chu4", '级': "ji5", '课': "ke2", '视': "shi4",
	'频': "pin", '认': "ren2", '证': "zheng",
}

// ToneStrip removes trailing digit tone markers for search matching.
// E.g. "shi2" -> "shi", "ji3" -> "ji"
func ToneStrip(s string) string {
	if len(s) > 0 && s[len(s)-1] >= '2' && s[len(s)-1] <= '5' {
		return s[:len(s)-1]
	}
	return s
}

// ToPinyin converts a string containing Chinese characters to pinyin.
// Non-Chinese characters are passed through as-is (lowercased).
// Spaces are inserted between pinyin syllables for searchability.
func ToPinyin(input string) string {
	if input == "" {
		return ""
	}

	var result strings.Builder
	lastWasPinyin := false

	for _, r := range input {
		if py, ok := pinyinTable[r]; ok {
			if result.Len() > 0 && !lastWasPinyin {
				result.WriteRune(' ')
			} else if lastWasPinyin {
				result.WriteRune(' ')
			}
			result.WriteString(ToneStrip(py))
			lastWasPinyin = true
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if lastWasPinyin {
				result.WriteRune(' ')
			}
			result.WriteRune(unicode.ToLower(r))
			lastWasPinyin = false
		} else if r == ' ' {
			if result.Len() > 0 {
				result.WriteRune(' ')
			}
			lastWasPinyin = false
		}
	}

	return strings.TrimSpace(result.String())
}

// ContainsChinese checks if a string contains any Chinese characters.
func ContainsChinese(s string) bool {
	for _, r := range s {
		if _, ok := pinyinTable[r]; ok {
			return true
		}
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
