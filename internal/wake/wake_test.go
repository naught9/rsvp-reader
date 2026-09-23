package wake

import (
	"testing"
)

func TestHoldReleaseIdempotent(t *testing.T) {
	Release() // clean slate
	if Held() {
		t.Fatalf("released lock reports held")
	}
	Hold()
	if !Held() {
		t.Fatalf("held lock reports released")
	}
	Hold() // second hold must not duplicate
	if !Held() {
		t.Fatalf("double hold lost the lock")
	}
	Release()
	Release() // double release must not panic or underflow
	if Held() {
		t.Fatalf("released lock reports held")
	}
}
