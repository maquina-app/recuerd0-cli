package importer

import "sort"

func PlanDigest(plan *Plan) Digest {
	digest := Digest{
		Adapter:    plan.Adapter,
		Counts:     Counts{Excluded: plan.Scan.Excluded},
		Exceptions: append([]Exception(nil), plan.Exceptions...),
		Warnings:   append([]string(nil), plan.Scan.Warnings...),
	}
	for _, row := range plan.Manifest {
		switch row.Action {
		case ActionCreate:
			digest.Counts.Create++
		case ActionVersion:
			digest.Counts.Version++
		case ActionSkip:
			digest.Counts.Skip++
		}
		digest.TagsProposed += len(row.Tags)
	}
	for _, exception := range plan.Exceptions {
		switch exception.Kind {
		case "conflict":
			digest.Counts.Conflicts++
		case "unparseable":
			digest.Counts.Unparseable++
		}
	}
	if plan.Adapter == AdapterWorkspaceExport {
		digest.TitlesFromH1Pct = 100
		digest.Thin = false
	} else {
		if plan.Scan.TitlesTotal > 0 {
			digest.TitlesFromH1Pct = plan.Scan.TitlesFromH1 * 100 / plan.Scan.TitlesTotal
		}
		digest.LinksProposed = countLinkPairs(plan.Manifest)
		digest.Thin = len(plan.Manifest) > 0 &&
			digest.TitlesFromH1Pct < 50 &&
			digest.LinksProposed == 0 &&
			digest.TagsProposed == 0
		if digest.Thin {
			digest.Hint = ThinHint
		}
	}
	if digest.Exceptions == nil {
		digest.Exceptions = []Exception{}
	}
	if digest.Warnings == nil {
		digest.Warnings = []string{}
	}
	return digest
}

func countLinkPairs(rows []PlanRow) int {
	paths := make(map[string]bool, len(rows))
	for _, row := range rows {
		paths[row.Path] = true
	}
	pairs := make(map[string]bool)
	for _, row := range rows {
		for _, target := range row.Links {
			if target == row.Path || !paths[target] {
				continue
			}
			ends := []string{row.Path, target}
			sort.Strings(ends)
			pairs[ends[0]+"\x00"+ends[1]] = true
		}
	}
	return len(pairs)
}

func sortExceptions(exceptions []Exception) {
	sort.Slice(exceptions, func(i, j int) bool {
		if exceptions[i].Path != exceptions[j].Path {
			return exceptions[i].Path < exceptions[j].Path
		}
		if exceptions[i].Kind != exceptions[j].Kind {
			return exceptions[i].Kind < exceptions[j].Kind
		}
		return exceptions[i].Detail < exceptions[j].Detail
	})
}
