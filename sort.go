package livepkg

import (
	"errors"
	"fmt"
)

type byType []*File

func (a byType) Len() int           { return len(a) }
func (a byType) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byType) Less(i, j int) bool { return a[i].Ext < a[j].Ext }

func sortFiles(initial []*File) ([]*File, error) {
	order, err := sortByDeps(initial)
	// sort.Sort(byType(order))
	return order, err
}

// sortByDeps sorts files as best as possible
// missing files will be ignored in sorting and error returned
// in presence of import cycles the cyclic files will be randomized
func sortByDeps(initial []*File) (order []*File, err error) {
	missing := []string{}
	sorted := make(map[string]bool)
	order = []*File{}

	files := make(map[string]*File, len(initial))
	for _, file := range initial {
		files[file.Path] = file
	}

	// brute-force topological sort
	for pass := 0; pass < 100; pass += 1 {
		changes := false
	next:
		for _, file := range files {
			if sorted[file.Path] {
				continue
			}

			for _, dep := range file.Deps {
				if sorted[dep] {
					continue
				}
				if _, exists := files[dep]; !exists {
					sorted[dep] = true
					missing = append(missing, dep)
					continue
				}
				continue next
			}

			changes = true
			sorted[file.Path] = true
			order = append(order, file)
		}

		if len(order) == len(files) {
			if len(missing) > 0 {
				return order, fmt.Errorf("some files are missing: %v", missing)
			}
			return order, nil
		}

		// found an import loop
		if !changes {
			break
		}
	}

	unsorted := []string{}
	for _, file := range files {
		if sorted[file.Path] {
			continue
		}

		unsorted = append(unsorted, file.Path)
		order = append(order, file)
	}

	errtext := fmt.Sprintf("unable to sort some items: %s", unsorted)
	if len(missing) > 0 {
		errtext += fmt.Sprintf("some files are missing: %s", missing)
	}

	return order, errors.New(errtext)
}
