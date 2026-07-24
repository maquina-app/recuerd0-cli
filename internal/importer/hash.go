package importer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

func sha256String(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// CanonicalTupleHash hashes the content tuple used by plans and ledger records.
func CanonicalTupleHash(title string, tags []string, category, body string) string {
	return sha256String(canonicalTuple(title, tags, category, body))
}

func canonicalTuple(title string, tags []string, category, body string) []byte {
	sortedTags := append([]string(nil), tags...)
	sort.Strings(sortedTags)
	return []byte(title + "\n" + strings.Join(sortedTags, ",") + "\n" + category + "\n" + body)
}

func sourceBodyHash(body string) string {
	return sha256String([]byte(body))
}

func exportSourceHash(versions []VersionData) string {
	h := sha256.New()
	for _, version := range versions {
		serialized := canonicalTuple(version.Title, version.Tags, version.Category, version.Body)
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(serialized)))
		_, _ = h.Write(length[:])
		_, _ = h.Write(serialized)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func rowFingerprint(title, category string, tags, links []string) string {
	sortedTags := append([]string(nil), tags...)
	sortedLinks := append([]string(nil), links...)
	sort.Strings(sortedTags)
	sort.Strings(sortedLinks)
	value := struct {
		Title    string   `json:"title"`
		Category string   `json:"category"`
		Tags     []string `json:"tags"`
		Links    []string `json:"links"`
	}{
		Title: title, Category: category, Tags: sortedTags, Links: sortedLinks,
	}
	data, _ := json.Marshal(value)
	return sha256String(data)
}
