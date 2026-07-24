package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// scanWorkspaceExport accepts only the server's workspace-export v1 envelope.
// Roots and version chains are validated strictly after decoding the envelope.
func scanWorkspaceExport(sourcePath string) (*scanResult, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read workspace export: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]interface{}
	if err := decoder.Decode(&root); err != nil {
		return nil, validationf("workspace_export source is not valid JSON: %v", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err == nil {
		return nil, validationf("workspace_export source contains multiple JSON values")
	} else if err != io.EOF {
		return nil, validationf("workspace_export source has trailing invalid JSON: %v", err)
	}

	version := firstInt(root, "format_version")
	format, _ := root["format"].(string)
	if format != "recuerd0.workspace_export" || version != 1 {
		return nil, validationf("workspace_export must be export v1")
	}
	workspace, ok := root["workspace"].(map[string]interface{})
	if !ok {
		return nil, validationf("workspace_export is missing a positive source workspace ID")
	}
	workspaceID := firstInt64(workspace, "id")
	if workspaceID <= 0 {
		return nil, validationf("workspace_export is missing a positive source workspace ID")
	}

	payload := root["memories"]
	items, ok := payload.([]interface{})
	if !ok {
		return nil, validationf("workspace_export memories must be an array")
	}
	rows, err := decodeExportRows(workspaceID, items)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].RootID < rows[j].RootID })
	return &scanResult{
		Rows: rows,
		Stats: ScanStats{
			TitlesTotal: len(rows), TitlesFromH1: len(rows),
			Warnings: []string{"workspace_export does not include memory links; links were omitted"},
		},
		Adapter: AdapterWorkspaceExport,
	}, nil
}

func decodeExportRows(workspaceID int64, items []interface{}) ([]sourceRow, error) {
	grouped := make(map[int64][]map[string]interface{})
	for i, item := range items {
		object, ok := item.(map[string]interface{})
		if !ok {
			return nil, validationf("workspace_export memory %d must be an object", i+1)
		}
		rootID := firstInt64(object, "root_id", "memory_id")
		versionsValue, hasVersions := object["versions"]
		if !hasVersions {
			// Also support the server's flat, one-record-per-version payload.
			if rootID == 0 {
				rootID = firstInt64(object, "id")
			}
			if rootID <= 0 {
				return nil, validationf("workspace_export memory %d is missing root_id", i+1)
			}
			grouped[rootID] = append(grouped[rootID], object)
			continue
		}
		if rootID == 0 {
			rootID = firstInt64(object, "id")
		}
		if rootID <= 0 {
			return nil, validationf("workspace_export memory %d is missing root_id", i+1)
		}
		if _, exists := grouped[rootID]; exists {
			return nil, validationf("workspace_export repeats root_id %d", rootID)
		}
		array, ok := versionsValue.([]interface{})
		if !ok || len(array) == 0 {
			return nil, validationf("workspace_export root %d versions must be a non-empty array", rootID)
		}
		for ordinal, raw := range array {
			version, ok := raw.(map[string]interface{})
			if !ok {
				return nil, validationf("workspace_export root %d version %d must be an object", rootID, ordinal+1)
			}
			grouped[rootID] = append(grouped[rootID], version)
		}
	}

	rootIDs := make([]int64, 0, len(grouped))
	for rootID := range grouped {
		rootIDs = append(rootIDs, rootID)
	}
	sort.Slice(rootIDs, func(i, j int) bool { return rootIDs[i] < rootIDs[j] })
	rows := make([]sourceRow, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		rawVersions := grouped[rootID]
		versions := make([]VersionData, 0, len(rawVersions))
		for i, raw := range rawVersions {
			number := firstInt(raw, "version", "ordinal")
			if number == 0 {
				number = i + 1
			}
			if number != i+1 {
				return nil, validationf("workspace_export root %d version chain is not contiguous at %d", rootID, i+1)
			}
			version, err := decodeExportVersion(rootID, i+1, raw)
			if err != nil {
				return nil, err
			}
			versions = append(versions, version)
		}
		latest := versions[len(versions)-1]
		rows = append(rows, sourceRow{
			Path:  fmt.Sprintf("workspace/%d/memories/%d", workspaceID, rootID),
			Title: latest.Title, Body: latest.Body,
			Tags: append([]string(nil), latest.Tags...), Category: latest.Category,
			SourceHash:        exportSourceHash(versions),
			SourceWorkspaceID: workspaceID, RootID: rootID, Versions: versions,
		})
	}
	return rows, nil
}

func decodeExportVersion(rootID int64, ordinal int, raw map[string]interface{}) (VersionData, error) {
	title, ok := raw["title"].(string)
	if !ok {
		return VersionData{}, validationf("workspace_export root %d version %d has invalid title", rootID, ordinal)
	}
	body := ""
	switch content := raw["content"].(type) {
	case string:
		body = content
	case map[string]interface{}:
		body, _ = content["body"].(string)
	default:
		body, _ = raw["body"].(string)
	}
	if _, contentExists := raw["content"]; !contentExists {
		body, _ = raw["body"].(string)
	}
	tags, err := stringSlice(raw["tags"])
	if err != nil {
		return VersionData{}, validationf("workspace_export root %d version %d tags: %v", rootID, ordinal, err)
	}
	category, _ := raw["category"].(string)
	if category == "" {
		category = "general"
	}
	if !validCategories[category] {
		return VersionData{}, validationf("workspace_export root %d version %d has invalid category %q", rootID, ordinal, category)
	}
	return VersionData{Title: title, Body: body, Tags: tags, Category: category}, nil
}

func firstInt(object map[string]interface{}, keys ...string) int {
	return int(firstInt64(object, keys...))
}

func firstInt64(object map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		switch value := object[key].(type) {
		case json.Number:
			number, _ := value.Int64()
			if number != 0 {
				return number
			}
		case float64:
			return int64(value)
		case int:
			return int64(value)
		case int64:
			return value
		case string:
			number, _ := strconv.ParseInt(value, 10, 64)
			if number != 0 {
				return number
			}
		}
	}
	return 0
}

func stringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return []string{}, nil
	}
	switch items := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("must contain only strings")
			}
			result = append(result, text)
		}
		return result, nil
	case []string:
		return append([]string(nil), items...), nil
	case string:
		if strings.TrimSpace(items) == "" {
			return []string{}, nil
		}
		return strings.Split(items, ","), nil
	default:
		return nil, fmt.Errorf("must be an array")
	}
}
