// Copyright (c) 2024 Erik Kassubek
//
// File: fuzzing.go
// Brief: Main file for fuzzing
//
// Author: Erik Kassubek
// Created: 2024-12-03
//
// License: BSD-3-Clause

package fuzzing

import (
	"advocate/analysis"
	"advocate/memory"
	"advocate/results"
	"advocate/stats"
	"advocate/timer"
	"advocate/toolchain"
	"advocate/utils"
	"fmt"
	"math"
	"path/filepath"
	"time"
)

// Encapsulating type for the different mutations
type mutation struct {
	mutType int
	mutSel  map[string][]fuzzingSelect
	mutFlow map[string]int
	mutPie  int
}

// Possible values for fuzzing mode
const (
	GFuzz       = "GFuzz"       // only GFuzz
	GFuzzHB     = "GFuzzHB"     // GFuzz with use of hb info
	GFuzzHBFlow = "GFuzzHBFlow" // GFuzz with use of hb info and flow mutation
	Flow        = "Flow"        // only flow mutation
	GoPie       = "GoPie"       // only goPie
	GoPiePlus   = "GoPie+"      // improved goPie without HB
	GoPieHB     = "GoPieHB"     // goPie with HB relation
)

const (
	mutSelType  = 0
	mutPiType   = 1
	mutFlowType = 2
)

const (
	maxRunPerMut = 2

	factorCaseWithPartner = 3
	maxFlowMut            = 10
)

var (
	maxNumberRuns     = 100
	maxTime           = 7 * time.Minute
	maxTimeSet        = false
	numberFuzzingRuns = 0
	mutationQueue     = make([]mutation, 0)
	// count how often a specific mutation has been in the queue
	allMutations     = make(map[string]int)
	fuzzingMode      = ""
	fuzzingModeGFuzz = false
	fuzzingModeGoPie = false
	fuzzingModeFlow  = false

	cancelTestIfBugFound = false
)

// Fuzzing creates the fuzzing data and runs the fuzzing executions
//
// Parameter:
//   - modeMain bool: if true, run fuzzing on main function, otherwise on test
//   - fm bool: the mode used for fuzzing
//   - advocate string: path to advocate
//   - progPath string: path to the folder containing the prog/test
//   - progName string: name of the program
//   - name string: If modeMain, name of the executable, else name of the test
//   - ignoreAtomic bool: if true, ignore atomics for replay
//   - meaTime bool: measure runtime
//   - notExec bool: find never executed operations
//   - stats bool: create statistics
//   - keepTraces bool: keep the traces after analysis
//   - skipExisting bool: skip existing runs
//   - cont bool: continue partial fuzzing
//   - mTime int: maximum time in seconds spend for one test/prog\
//   - mRun int: maximum number of times a test/prog is run
//   - cancelTestIfFound int: do not run further fuzzing runs on tests if one
//     bug has been found, mainly used for benchmarks
func Fuzzing(modeMain bool, fm, advocate, progPath, progName, name string, ignoreAtomic,
	meaTime, notExec, createStats, keepTraces, cont bool, mTime, mRun int,
	cancelTestIfFound bool) error {

	if fm == "" {
		return fmt.Errorf("No fuzzing mode selected. Select with -fuzzingMode [mode]. Possible values are GoPie, GoPie+, GoPieHB, GFuzz, GFuzzFlow, GFuzzHB, Flow")
	}

	modes := []string{GoPie, GoPiePlus, GoPieHB, GFuzz, GFuzzHBFlow, GFuzzHB, Flow}
	if !utils.Contains(modes, fm) {
		return fmt.Errorf("Invalid fuzzing mode '%s'. Possible values are GoPie, GoPie+, GoPieHB, GFuzz, GFuzzFlow, GFuzzHB, Flow", fm)
	}

	maxNumberRuns = mRun
	if maxTime > 0 {
		maxTime = time.Duration(mTime) * time.Second
		maxTimeSet = true
	}

	fuzzingMode = fm
	fuzzingModeGoPie = (fuzzingMode == GoPie || fuzzingMode == GoPiePlus || fuzzingMode == GoPieHB)
	fuzzingModeGFuzz = (fuzzingMode == GFuzz || fuzzingMode == GFuzzHBFlow || fuzzingMode == GFuzzHB)
	fuzzingModeFlow = (fuzzingMode == Flow || fuzzingMode == GFuzzHBFlow)
	useHBInfoFuzzing = (fuzzingMode == GFuzzHB || fuzzingMode == GFuzzHBFlow || fuzzingMode == Flow || fuzzingMode == GoPieHB)

	cancelTestIfBugFound = cancelTestIfFound

	if cont {
		utils.LogInfo("Continue fuzzing")
	} else {
		utils.LogInfo("Start fuzzing")
	}

	// run either fuzzing on main or fuzzing on one test
	if modeMain || name != "" {
		if modeMain {
			utils.LogInfo("Run fuzzing on main function")
		} else {
			utils.LogInfo("Run fuzzing on test ", name)
		}

		clearData()
		timer.ResetFuzzing()
		memory.Reset()

		err := runFuzzing(modeMain, advocate, progPath, progName, "", name, ignoreAtomic,
			meaTime, notExec, createStats, keepTraces, true, cont, 0, 0)

		if createStats {
			err := stats.CreateStatsFuzzing(getPath(progPath), progName)
			if err != nil {
				utils.LogError("Failed to create fuzzing stats: ", err.Error())
			}
			err = stats.CreateStatsTotal(getPath(progPath), progName)
			if err != nil {
				utils.LogError("Failed to create total stats: ", err.Error())
			}
		}

		return err
	}

	utils.LogInfo("Run fuzzing on all tests")

	// run fuzzing on all tests
	testFiles, maxFileNumber, totalFiles, err := toolchain.FindTestFiles(progPath, cont)
	if err != nil {
		return fmt.Errorf("Failed to find test files: %v", err)
	}

	utils.LogInfof("Found %d test files", totalFiles)

	// Process each test file
	fileCounter := 0
	if cont {
		fileCounter = maxFileNumber
	}

	for i, testFile := range testFiles {
		fileCounter++
		utils.LogProgressf("Progress %s: %d/%d\n", progName, fileCounter, totalFiles)
		utils.LogProgressf("Processing file: %s\n", testFile)

		testFunctions, err := toolchain.FindTestFunctions(testFile)
		if err != nil || len(testFunctions) == 0 {
			utils.LogInfo("Could not find test functions in ", testFile)
			continue
		}

		for j, testFunc := range testFunctions {
			resetFuzzing()
			timer.ResetTest()
			memory.Reset()

			timer.Start(timer.TotalTest)

			utils.LogProgressf("Run fuzzing for %s->%s", testFile, testFunc)

			firstRun := (i == 0 && j == 0)

			err := runFuzzing(false, advocate, progPath, progName, testFile, testFunc, ignoreAtomic,
				meaTime, notExec, createStats, keepTraces, firstRun, cont, fileCounter, j+1)
			if err != nil {
				utils.LogError("Error in fuzzing: ", err.Error())
			}

			timer.Stop(timer.TotalTest)

			timer.UpdateTimeFileOverview(progName, testFunc)
		}

	}

	if createStats {
		err := stats.CreateStatsFuzzing(getPath(progPath), progName)
		if err != nil {
			utils.LogError("Failed to create fuzzing stats: ", err.Error())
		}
		err = stats.CreateStatsTotal(getPath(progPath), progName)
		if err != nil {
			utils.LogError("Failed to create total stats: ", err.Error())
		}
	}

	return nil
}

// Run Fuzzing on one program/test
//
// Parameter:
//   - modeMain bool: if true, run fuzzing on main function, otherwise on test
//   - advocate string: path to advocate
//   - progName string: name of the program
//   - testPath string: path to the test file
//   - name string: If modeMain, name of the executable, else name of the test
//   - ignoreAtomic bool: if true, ignore atomics for replay
//   - hBInfoFuzzing bool: whether to us HB info in fuzzing
//   - meaTime bool: measure runtime
//   - notExec bool: find never executed operations
//   - createStats bool: create statistics
//   - keepTraces bool: keep the traces after analysis
//   - skipExisting bool: skip existing runs
//   - firstRun bool: this is the first run, only set to false for fuzzing (except for the first fuzzing)
//   - cont bool: continue with an already started run
func runFuzzing(modeMain bool, advocate, progPath, progName, testPath, name string, ignoreAtomic,
	meaTime, notExec, createStats, keepTraces, firstRun, cont bool, fileNumber, testNumber int) error {

	progDir := getPath(progPath)

	clearDataFull()

	startTime := time.Now()

	// while there are available mutations, run them
	for numberFuzzingRuns == 0 || len(mutationQueue) != 0 {

		// clean up
		clearData()
		timer.ResetFuzzing()
		memory.Reset()

		if cancelTestIfBugFound && results.GetBugWasFound() {
			utils.LogResultf(false, false, "", "Cancel test after %d runs", numberFuzzingRuns)
			break
		}

		utils.LogInfo("Fuzzing Run: ", numberFuzzingRuns+1)

		fuzzingPath := ""
		progPathDir := utils.GetDirectory(progPath)
		var order mutation
		if numberFuzzingRuns != 0 {
			order = popMutation()
			if order.mutType == mutPiType {
				fuzzingPath = filepath.Join(progPathDir,
					filepath.Join("fuzzingTraces",
						fmt.Sprintf("fuzzingTrace_%d", order.mutPie)))
			} else {
				err := writeMutationToFile(progPathDir, order)
				if err != nil {
					return err
				}
			}
		}

		firstRun = firstRun && (numberFuzzingRuns == 0)

		// Run the test/mutation

		mode := "test"
		if modeMain {
			mode = "main"
		}
		err := toolchain.Run(mode, advocate, progPath, testPath, true, true, true,
			name, progName, name, numberFuzzingRuns, fuzzingPath, ignoreAtomic,
			meaTime, notExec, createStats, keepTraces, false, firstRun, cont,
			fileNumber, testNumber)
		if err != nil {
			utils.LogError("Fuzzing run failed: ", err.Error())
		} else {
			// collect the required data to decide whether run is interesting
			// and to create the mutations
			ParseTrace(&analysis.MainTrace)

			if memory.WasCanceled() {
				numberFuzzingRuns++
				continue
			}

			utils.LogInfof("Create mutations")
			if fuzzingModeGFuzz {
				utils.LogInfof("Create GFuzz mutations")
				createGFuzzMut()
			}

			// add new mutations based on flow path expansion
			if fuzzingModeFlow {
				utils.LogInfof("Create Flow mutations")
				createMutationsFlow()
			}

			// add mutations based on GoPie
			if fuzzingModeGoPie {
				utils.LogInfof("Create GoPie mutations")
				createGoPieMut(progDir, numberFuzzingRuns, order.mutPie)
			}

			utils.LogInfof("Current fuzzing queue size: %d", len(mutationQueue))

			mergeTraceInfoIntoFileInfo()
		}

		numberFuzzingRuns++

		// cancel if max number of mutations have been reached
		if maxNumberRuns != -1 && numberFuzzingRuns >= maxNumberRuns {
			utils.LogInfof("Finish fuzzing because maximum number of mutation runs (%d) have been reached", maxNumberRuns)
			return nil
		}

		if maxTimeSet && time.Since(startTime) > maxTime {
			utils.LogInfof("Finish fuzzing because maximum runtime for fuzzing (%d min)has been reached", int(maxTime.Minutes()))
			return nil
		}
	}

	if fuzzingModeGoPie {
		toolchain.ClearFuzzingTrace(progDir, keepTraces)
	}

	utils.LogInfof("Finish fuzzing after %d runs\n", numberFuzzingRuns)

	return nil
}

// Remove and return the first mutation from the mutation queue
//
// Returns:
//   - the first mutation from the mutation queue
func popMutation() mutation {
	var mut mutation
	mut, mutationQueue = mutationQueue[0], mutationQueue[1:]
	return mut
}

// Get the probability that a select changes its preferred case
// It is selected in such a way, that at least one of the selects if flipped
// with a probability of at least 99%.
// Additionally the flip probability is at least 10% for each select.
func getFlipProbability() float64 {
	p := 0.99   // min prob that at least one case is flipped
	pMin := 0.1 // min prob that a select is flipt

	return max(pMin, 1-math.Pow(1-p, 1/float64(numberSelects)))
}

// Reset fuzzing
func resetFuzzing() {
	numberFuzzingRuns = 0
	mutationQueue = make([]mutation, 0)
	// count how often a specific mutation has been in the queue
	allMutations = make(map[string]int)
}

// Add a mutation to the queue. If a maximum number of mutation runs in set,
// only add the mutation if it does not exceed this max number
//
// Parameter:
//   - mut mutation: the mutation to add
func addMutToQueue(mut mutation) {
	if maxNumberRuns == -1 || numberFuzzingRuns+len(mutationQueue) <= maxNumberRuns {
		mutationQueue = append(mutationQueue, mut)
	}
}
