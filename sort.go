package livepkg

import (
	"bytes"
	"errors"
	"fmt"
	"text/tabwriter"
)

type byType []*Source

func (a byType) Len() int           { return len(a) }
func (a byType) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byType) Less(i, j int) bool { return a[i].Ext < a[j].Ext }

// SortSources optimistically sorts items by dependencies
// in case of cycles those items will be unordered
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

	// there is a problem with some of the unsorted items
	for _, src := range sources {
		if sorted[src.Path] {
			continue
		}
		order = append(order, src)
	}

	help := formatUnsorted(sources, sorted)
	errtext := fmt.Sprintf("cycle in dependencies:\n%s\n", help)
	if len(missing) > 0 {
		errtext += fmt.Sprintf("some sources are missing: %s", missing)
	}

	return order, errors.New(errtext)
}

func formatUnsorted(sources map[string]*Source, sorted map[string]bool) string {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 8, 0, '\t', 0)

	for _, src := range sources {
		if sorted[src.Path] {
			continue
		}

		for _, dep := range src.Deps {
			if !sorted[dep] {
				fmt.Fprintf(tw, "    \t%s\t->\t%s\n", src.Path, dep)
			}
		}
	}
	tw.Flush()

	return buf.String()
}
