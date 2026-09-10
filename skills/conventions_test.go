package skills

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

const expectedConventionsSHA256 = "0871ae8547fd30dab6b05a63e3592e99a9019a316a71087566a626762acc67db"

func TestConventionsMatchReviewedArtifact(t *testing.T) {
	data, err := os.ReadFile("recuerd0/references/conventions.md")
	if err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != expectedConventionsSHA256 {
		t.Fatalf("conventions SHA-256 = %s, want %s", got, expectedConventionsSHA256)
	}
}
