package telkomsel

import (
	"fmt"
	"testing"
)

func TestGenerateSignatureNew(t *testing.T) {
	timestamp := unixNowUTC()
	fmt.Println("Timestamp:", timestamp)
	sig, err := generateSignature(timestamp)
	if err != nil {
		t.Fatalf("generateSignature error: %v", err)
	}

	fmt.Println("Signature:", sig)
}
