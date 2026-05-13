package variables

import (
	"sort"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

// MergeAllScopes folds org → env → component variables into a single list,
// with later tiers overriding earlier ones on name collisions. Each returned
// entry is the original Variable from whichever tier won, so its Type field
// already identifies the winning scope.
//
// This is the same precedence the controller applies at render time
// (see applications/builder.go gatherVariables) — keeping a single helper
// guarantees the "list" view and the rendered Component CR cannot drift.
func MergeAllScopes(orgVars, envVars, compVars []models.Variable) []models.Variable {
	byName := make(map[string]models.Variable, len(orgVars)+len(envVars)+len(compVars))
	for _, tier := range [][]models.Variable{orgVars, envVars, compVars} {
		for _, v := range tier {
			byName[v.Name] = v
		}
	}
	out := make([]models.Variable, 0, len(byName))
	for _, v := range byName {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
