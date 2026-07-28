package skills

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

const expectedConventionsSHA256 = "6771db25678e7fd556d5bed62527cd713d3c7f0cafd43269e4242441465f1ddd"

func TestConventionsMatchReviewedArtifact(t *testing.T) {
	data, err := os.ReadFile("recuerd0/references/conventions.md")
	if err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != expectedConventionsSHA256 {
		t.Fatalf("conventions SHA-256 = %s, want %s", got, expectedConventionsSHA256)
	}
}
