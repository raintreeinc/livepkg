package livepkg

import (
	"errors"
	"fmt"
)

type byType []*Source

func (a byType) Len() int           { return len(a) }
func (a byType) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byType) Less(i, j int) bool { return a[i].Ext < a[j].Ext }

func SortSources(sources []*Source) ([]*Source, error) {
	order, err := sortByDeps(sources)
	// sort.Sort(byType(order))
	return order, err
}

// sortByDeps sorts files as best as possible
// missing files will be ignored in sorting and error returned
// in presence of import cycles the cyclic files will be randomized
func sortByDeps(initial []*Source) (order []*Source, err error) {
	missing := []string{}
	sorted := make(map[string]bool)
	order = []*Source{}

	sources := make(map[string]*Source, len(initial))
	for _, src := range initial {
		sources[src.Path] = src
	}

	// brute-force topological sort
	for pass := 0; pass < 100; pass += 1 {
		changes := false
	next:
		for _, src := range sources {
			if sorted[src.Path] {
				continue
			}

			for _, dep := range src.Deps {
				if sorted[dep] {
					continue
				}
				if _, exists := sources[dep]; !exists {
					sorted[dep] = true
					missing = append(missing, dep)
					continue
				}
				continue next
			}

			changes = true
			sorted[src.Path] = true
			order = append(order, src)
		}

		if len(order) == len(sources) {
			if len(missing) > 0 {
				return order, fmt.Errorf("some sources are missing: %v", missing)
			}
			return order, nil
		}

		// found an import loop
		if !changes {
			break
		}
	}

	unsorted := []string{}
	for _, src := range sources {
		if sorted[src.Path] {
			continue
		}

		unsorted = append(unsorted, src.Path)
		order = append(order, src)
	}

	errtext := fmt.Sprintf("unable to sort some items: %s", unsorted)
	if len(missing) > 0 {
		errtext += fmt.Sprintf("some sources are missing: %s", missing)
	}

	return order, errors.New(errtext)
}
