// Copyright (c) 2024 Erik Kassubek
//
// File: analysisWaitGroup.go
// Brief: Trace analysis for possible negative wait group counter
//
// Author: Erik Kassubek
// Created: 2023-11-24
//
// License: BSD-3-Clause

package analysis

import (
	"advocate/clock"
	"advocate/results"
	"advocate/timer"
	"advocate/trace"
	"advocate/utils"
	"fmt"
)

// Collect all adds and dones for the analysis
//
// Parameter:
//   - wa *TraceElementWait: the trace wait or done element
func checkForDoneBeforeAddChange(wa *trace.TraceElementWait) {
	timer.Start(timer.AnaWait)
	defer timer.Stop(timer.AnaWait)

	delta := wa.GetDelta()

	if delta > 0 {
		checkForDoneBeforeAddAdd(wa)
	} else if delta < 0 {
		checkForDoneBeforeAddDone(wa)
	} else {
		// checkForImpossibleWait(routine, id, pos, vc)
	}
}

// Collect all adds for the analysis
//
// Parameter:
//   - wa *TraceElementWait: the trace wait element
func checkForDoneBeforeAddAdd(wa *trace.TraceElementWait) {
	id := wa.GetID()

	// if necessary, create maps and lists
	if _, ok := wgAdd[id]; !ok {
		wgAdd[id] = make([]trace.TraceElement, 0)
	}

	// add the vector clock and position to the list
	wgAdd[id] = append(wgAdd[id], wa)
}

// Collect all dones for the analysis
//
// Parameter:
//   - wa *TraceElementWait: the trace done element
func checkForDoneBeforeAddDone(wa *trace.TraceElementWait) {
	id := wa.GetID()

	// if necessary, create maps and lists
	if _, ok := wgDone[id]; !ok {
		wgDone[id] = make([]trace.TraceElement, 0)

	}

	// add the vector clock and position to the list
	wgDone[id] = append(wgDone[id], wa)
}

// Check if a wait group counter could become negative
// For each done operation, build a bipartite st graph.
// Use the Ford-Fulkerson algorithm to find the maximum flow.
// If the maximum flow is smaller than the number of done operations, a negative wait group counter is possible.
func checkForDoneBeforeAdd() {
	timer.Start(timer.AnaWait)
	defer timer.Stop(timer.AnaWait)

	for id := range wgAdd { // for all waitgroups
		graph := buildResidualGraph(wgAdd[id], wgDone[id])

		maxFlow, graph, err := calculateMaxFlow(graph)
		if err != nil {
			fmt.Println("Could not check for done before add: ", err)
		}
		nrDone := len(wgDone[id])

		addsNegWg := make([]trace.TraceElement, 0)
		donesNegWg := make([]trace.TraceElement, 0)

		if maxFlow < nrDone {
			// sort the adds and dones, that do not have a partner is such a way,
			// that the i-th add in the result message is concurrent with the
			// i-th done in the result message

			for _, add := range wgAdd[id] {
				if !utils.Contains(graph[drain], add) {
					addsNegWg = append(addsNegWg, add)
				}
			}

			for _, dones := range graph[source] {
				donesNegWg = append(donesNegWg, dones)
			}

			addsNegWgSorted := make([]trace.TraceElement, 0)
			donesNEgWgSorted := make([]trace.TraceElement, 0)

			for i := 0; i < len(addsNegWg); {
				removed := false
				for j := 0; j < len(donesNegWg); {
					if clock.GetHappensBefore(addsNegWg[i].GetVC(), donesNegWg[j].GetVC()) == clock.Concurrent {
						addsNegWgSorted = append(addsNegWgSorted, addsNegWg[i])
						donesNEgWgSorted = append(donesNEgWgSorted, donesNegWg[j])
						// remove the element from the list
						addsNegWg = append(addsNegWg[:i], addsNegWg[i+1:]...)
						donesNegWg = append(donesNegWg[:j], donesNegWg[j+1:]...)
						// fix the index
						removed = true
						break
					} else {
						j++
					}
				}
				if !removed {
					i++
				}
			}

			args1 := []results.ResultElem{} // dones
			args2 := []results.ResultElem{} // adds

			for _, done := range donesNEgWgSorted {
				if done.GetTID() == "\n" {
					continue
				}
				file, line, tPre, err := trace.InfoFromTID(done.GetTID())
				if err != nil {
					utils.LogError(err.Error())
					return
				}

				args1 = append(args1, results.TraceElementResult{
					RoutineID: done.GetRoutine(),
					ObjID:     id,
					TPre:      tPre,
					ObjType:   "WD",
					File:      file,
					Line:      line,
				})
			}

			for _, add := range addsNegWgSorted {
				if add.GetTID() == "\n" {
					continue
				}
				file, line, tPre, err := trace.InfoFromTID(add.GetTID())
				if err != nil {
					utils.LogError(err.Error())
					continue
				}

				args2 = append(args2, results.TraceElementResult{
					RoutineID: add.GetRoutine(),
					ObjID:     id,
					TPre:      tPre,
					ObjType:   "WA",
					File:      file,
					Line:      line,
				})

			}

			results.Result(results.CRITICAL, utils.PNegWG,
				"done", args1, "add", args2)
		}
	}
}
