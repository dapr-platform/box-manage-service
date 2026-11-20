package service

import "testing"

// TestIsAllDigits 验证数字识别，用于判断传入的是 workflow_id 还是 key_name。
func TestIsAllDigits(t *testing.T) {
	cases := map[string]bool{
		"393":   true,
		"418":   true,
		"cd51":  false,
		"995c":  false,
		"":      false,
		"12a34": false,
		" 393":  false,
	}
	for input, want := range cases {
		if got := isAllDigits(input); got != want {
			t.Fatalf("isAllDigits(%q) = %v, want %v", input, got, want)
		}
	}
}
