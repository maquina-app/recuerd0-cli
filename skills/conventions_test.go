package skills

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

const expectedConventionsSHA256 = "80748cff478b274716aecbc9ec0e8a434e52187dbee87fcfe7e4652ff2a33173"

func TestConventionsMatchReviewedArtifact(t *testing.T) {
	data, err := os.ReadFile("recuerd0/references/conventions.md")
	if err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != expectedConventionsSHA256 {
		t.Fatalf("conventions SHA-256 = %s, want %s", got, expectedConventionsSHA256)
	}
}
