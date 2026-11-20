package drivers

import (
	"errors"
	"testing"
)

func TestVKReplyIdentityAndPercent(t *testing.T) {
	report := []byte{0, 0x10, 0, 1, 0x0b, 'M', '2', '2', '0', 0x17, 3, 0x11, 1, 2, 100, 1}
	for _, b := range [][]byte{report, report[1:]} {
		if n, match, err := vkPercent(b); n != 100 || !match || err != nil {
			t.Fatalf("100%% reply: %d %t %v", n, match, err)
		}
	}
	report[14] = 0
	if n, match, err := vkPercent(report); n != 0 || !match || err != nil {
		t.Fatal("valid discharged sample lost")
	}
	copy(report[5:9], "E023")
	if _, _, err := vkPercent(report); !errors.Is(err, errDifferentVK) {
		t.Fatal("different VK model accepted")
	}
}
