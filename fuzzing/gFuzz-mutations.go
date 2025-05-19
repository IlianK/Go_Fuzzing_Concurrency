// Copyright (c) 2024 Erik Kassubek
//
// File: mutations.go
// Brief: Create the mutations
//
// Author: Erik Kassubek
// Created: 2024-12-03
//
// License: BSD-3-Clause

package fuzzing

import (
	"fmt"
	"sort"
)

// createMutationsGFuzz creates the new mutations for a trace based on GFuzz
//
// Parameter:
//   - numberMutation int: number of mutation to create
//   - flipChance float64: probability that for a given select the preferred case is changed
//
// Returns:
//   - int: number of added mutations
func createMutationsGFuzz(numberMutations int, flipChance float64) int {
	numberMutAdded := 0

	for i := 0; i < numberMutations; i++ {
		mut := createMutation(flipChance)

		id := getIDFromMut(mut)
		if id == "" {
			continue
		}

		if num, _ := allMutations[id]; num < maxRunPerMut {
			mut := mutation{mutType: mutSelType, mutSel: mut}
			addMutToQueue(mut)
			allMutations[id]++
			numberMutAdded++
		}
	}

	return numberMutAdded
}

// createMutation creates one new mutation
//
// Parameter:
//   - flipChance float64: probability that a select changes its preferred case
//
// Returns:
//   - map[string][]fuzzingSelect: the new mutation
func createMutation(flipChance float64) map[string][]fuzzingSelect {
	res := make(map[string][]fuzzingSelect)

	for key, listSel := range selectInfoTrace {
		res[key] = make([]fuzzingSelect, 0)
		for _, sel := range listSel {
			res[key] = append(res[key], sel.getCopyRandom(true, flipChance))
		}
	}

	return res
}

// Get a unique string id for a given mutation
//
// Parameter:
//   - mut map[string][]fuzzingSelect: mutation
//
// Returns:
//   - string: id
func getIDFromMut(mut map[string][]fuzzingSelect) string {
	keys := make([]string, 0, len(mut))
	for key := range mut {
		keys = append(keys, key)
	}

	// Sort the keys alphabetically
	sort.Strings(keys)

	id := ""

	// Iterate over the sorted keys
	for _, key := range keys {
		id := key + "-"
		for _, sel := range mut[key] {
			id += fmt.Sprintf("%d", sel.chosenCase)
		}
	}

	return id
}
