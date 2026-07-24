package importer

import (
	"errors"
	"fmt"
	"sort"

	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
)

type linkPair struct {
	firstPath  string
	secondPath string
	firstID    int64
	secondID   int64
}

func (runner *commitRunner) ensureLinks() {
	pairs, unresolved := linkPairs(runner.prepared.plan, runner.prepared.ledger)
	runner.summary.LinksSkippedUnresolvable = unresolved
	linkCache := make(map[int64]map[int64]bool)
	cacheLoaded := make(map[int64]bool)
	for _, pair := range pairs {
		path := fmt.Sprintf("/workspaces/%d/memories/%d/links", runner.prepared.plan.Workspace, pair.firstID)
		body := map[string]interface{}{"to_memory_id": pair.secondID}
		_, err := runner.api.Post(path, body)
		if err == nil {
			runner.summary.LinksEnsured.Created++
			continue
		}
		present := runner.linkPresent(pair.firstID, pair.secondID, linkCache, cacheLoaded)
		if present {
			runner.summary.LinksEnsured.Existing++
			continue
		}
		if !retryable(err) && httpStatus(err) != 422 {
			runner.summary.LinksFailed = append(runner.summary.LinksFailed, LinkFailure{
				FromPath: pair.firstPath, ToPath: pair.secondPath, Reason: err.Error(),
			})
			continue
		}
		_, retryErr := runner.api.Post(path, body)
		if retryErr == nil {
			runner.summary.LinksEnsured.Created++
			continue
		}
		present = runner.linkPresent(pair.secondID, pair.firstID, linkCache, cacheLoaded)
		if present {
			runner.summary.LinksEnsured.Existing++
			continue
		}
		runner.summary.LinksFailed = append(runner.summary.LinksFailed, LinkFailure{
			FromPath: pair.firstPath, ToPath: pair.secondPath, Reason: retryErr.Error(),
		})
	}
}

func linkPairs(plan *Plan, ledger *Ledger) ([]linkPair, int) {
	rows := make(map[string]PlanRow, len(plan.Manifest))
	for _, row := range plan.Manifest {
		rows[row.Path] = row
	}
	type pathPair struct{ first, second string }
	unique := make(map[pathPair]bool)
	for _, row := range plan.Manifest {
		for _, target := range row.Links {
			if target == row.Path {
				continue
			}
			if _, exists := rows[target]; !exists {
				continue
			}
			ends := []string{row.Path, target}
			sort.Strings(ends)
			unique[pathPair{ends[0], ends[1]}] = true
		}
	}
	pathPairs := make([]pathPair, 0, len(unique))
	for pair := range unique {
		pathPairs = append(pathPairs, pair)
	}
	sort.Slice(pathPairs, func(i, j int) bool {
		if pathPairs[i].first != pathPairs[j].first {
			return pathPairs[i].first < pathPairs[j].first
		}
		return pathPairs[i].second < pathPairs[j].second
	})
	var result []linkPair
	unresolved := 0
	for _, pair := range pathPairs {
		firstState, secondState := ledger.Paths[pair.first], ledger.Paths[pair.second]
		if firstState == nil || secondState == nil || firstState.MemoryID == 0 || secondState.MemoryID == 0 {
			unresolved++
			continue
		}
		result = append(result, linkPair{
			firstPath: pair.first, secondPath: pair.second,
			firstID: firstState.MemoryID, secondID: secondState.MemoryID,
		})
	}
	return result, unresolved
}

func (runner *commitRunner) linkPresent(memoryID, otherID int64, cache map[int64]map[int64]bool, loaded map[int64]bool) bool {
	if loaded[memoryID] {
		return cache[memoryID][otherID]
	}
	loaded[memoryID] = true
	cache[memoryID] = make(map[int64]bool)
	response, err := runner.api.Get(fmt.Sprintf("/workspaces/%d/memories/%d/links", runner.prepared.plan.Workspace, memoryID))
	if err != nil {
		return false
	}
	for _, item := range responseItems(response.Data) {
		memory, err := decodeMemory(item)
		if err == nil && memory.ID > 0 {
			cache[memoryID][memory.ID] = true
		}
	}
	return cache[memoryID][otherID]
}

func httpStatus(err error) int {
	var cliError *clierrors.CLIError
	if errors.As(err, &cliError) {
		return cliError.Status
	}
	return 0
}
