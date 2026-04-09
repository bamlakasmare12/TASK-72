package pinyin

import (
	"strings"
	"testing"
)

func TestToPinyin_PureChinese(t *testing.T) {
	result := ToPinyin("云计算")
	// 云 -> "yun", 计 -> "ji", 算 -> "suan"
	if !strings.Contains(result, "yun") || !strings.Contains(result, "ji") || !strings.Contains(result, "suan") {
		t.Errorf("ToPinyin(\"云计算\") = %q, expected it to contain \"yun\", \"ji\", and \"suan\"", result)
	}
}

func TestToPinyin_MixedInput(t *testing.T) {
	result := ToPinyin("Go编程")
	if !strings.Contains(result, "go") {
		t.Errorf("ToPinyin(\"Go编程\") = %q, expected it to contain \"go\"", result)
	}
	if !strings.Contains(result, "bian") {
		t.Errorf("ToPinyin(\"Go编程\") = %q, expected it to contain \"bian\"", result)
	}
	// 程 -> "cheng"
	if !strings.Contains(result, "cheng") {
		t.Errorf("ToPinyin(\"Go编程\") = %q, expected it to contain \"cheng\"", result)
	}
}

func TestToPinyin_PureEnglish(t *testing.T) {
	result := ToPinyin("hello world")
	if result != "hello world" {
		t.Errorf("ToPinyin(\"hello world\") = %q, expected \"hello world\"", result)
	}
}

func TestToPinyin_EmptyString(t *testing.T) {
	result := ToPinyin("")
	if result != "" {
		t.Errorf("ToPinyin(\"\") = %q, expected \"\"", result)
	}
}

func TestContainsChinese_WithChinese(t *testing.T) {
	if !ContainsChinese("云计算") {
		t.Error("ContainsChinese(\"云计算\") = false, expected true")
	}
}

func TestContainsChinese_WithoutChinese(t *testing.T) {
	if ContainsChinese("hello") {
		t.Error("ContainsChinese(\"hello\") = true, expected false")
	}
}

func TestContainsChinese_Mixed(t *testing.T) {
	if !ContainsChinese("Go编程") {
		t.Error("ContainsChinese(\"Go编程\") = false, expected true")
	}
}

func TestContainsChinese_Empty(t *testing.T) {
	if ContainsChinese("") {
		t.Error("ContainsChinese(\"\") = true, expected false")
	}
}

func TestToneStrip_WithTone(t *testing.T) {
	result := toneStrip("shi2")
	if result != "shi" {
		t.Errorf("toneStrip(\"shi2\") = %q, expected \"shi\"", result)
	}
}

func TestToneStrip_WithoutTone(t *testing.T) {
	result := toneStrip("shi")
	if result != "shi" {
		t.Errorf("toneStrip(\"shi\") = %q, expected \"shi\"", result)
	}
}

func TestToneStrip_SingleChar(t *testing.T) {
	result := toneStrip("a")
	if result != "a" {
		t.Errorf("toneStrip(\"a\") = %q, expected \"a\"", result)
	}
}
