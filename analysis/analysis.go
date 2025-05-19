// Copyright (c) 2024 Erik Kassubek
//
// File: analysis.go
// Brief: analysis of traces if performed from here
//
// Author: Erik Kassubek, Sebastian Pohsner
// Created: 2025-01-01
//
// License: BSD-3-Clause

package analysis

import (
	"advocate/clock"
	"advocate/memory"
	"advocate/results"
	"advocate/timer"
	"advocate/trace"
	"advocate/utils"
)

// RunAnalysis starts the analysis of the main trace
//
// Parameter:
//   - assume_fifo bool: True to assume fifo ordering in buffered channels
//   - ignoreCriticalSections bool: True to ignore critical sections when updating vector clocks
//   - analysisCasesMap map[string]bool: The analysis cases to run
//   - fuzzing bool: true if run with fuzzing
//   - onlyAPanicAndLeak bool: only test for actual panics and leaks
func RunAnalysis(assumeFifo bool, ignoreCriticalSections bool,
	analysisCasesMap map[string]bool, fuzzing bool, onlyAPanicAndLeak bool) {
	// catch panics in analysis.
	// Prevents the whole toolchain to panic if one analysis panics
	if utils.IsPanicPrevent() {
		defer func() {
			if r := recover(); r != nil {
				memory.Cancel()
				utils.LogError(r)
			}
		}()
	}

	timer.Start(timer.Analysis)
	defer timer.Stop(timer.Analysis)

	if onlyAPanicAndLeak {
		runAnalysisOnExitCodes(true)
		checkForStuckRoutine(true)
		return
	}

	runAnalysisOnExitCodes(fuzzing)
	RunHBAnalysis(assumeFifo, ignoreCriticalSections, analysisCasesMap, fuzzing)

	return
}

// runAnalysisOnExitCodes checks the exit codes for the recording for actual bugs
//
// Parameter:
//   - all bool: If true, check for all, else only check for the once, that are not detected by the full analysis
func runAnalysisOnExitCodes(all bool) {
	timer.Start(timer.AnaExitCode)
	defer timer.Stop(timer.AnaExitCode)

	switch exitCode {
	case utils.ExitCodeCloseClose: // close on closed
		file, line, err := trace.PosFromPosString(exitPos)
		if err != nil {
			utils.LogError("Could not read exit pos: ", err)
		}

		arg1 := results.TraceElementResult{
			RoutineID: 0,
			ObjID:     0,
			TPre:      0,
			ObjType:   "CC",
			File:      file,
			Line:      line,
		}
		results.Result(results.CRITICAL, utils.ACloseOnClosed,
			"close", []results.ResultElem{arg1}, "", []results.ResultElem{})
		bugWasFound = true
	case utils.ExitCodeCloseNil: // close on nil
		file, line, err := trace.PosFromPosString(exitPos)
		if err != nil {
			utils.LogError("Could not read exit pos: ", err)
		}
		arg1 := results.TraceElementResult{
			RoutineID: 0,
			ObjID:     0,
			TPre:      0,
			ObjType:   "CC",
			File:      file,
			Line:      line,
		}
		results.Result(results.CRITICAL, utils.ACloseOnNilChannel,
			"close", []results.ResultElem{arg1}, "", []results.ResultElem{})
		bugWasFound = true
	case utils.ExitCodeNegativeWG: // negative wg counter
		file, line, err := trace.PosFromPosString(exitPos)
		if err != nil {
			utils.LogError("Could not read exit pos: ", err)
		}
		arg1 := results.TraceElementResult{
			RoutineID: 0,
			ObjID:     0,
			TPre:      0,
			ObjType:   "WD",
			File:      file,
			Line:      line,
		}
		results.Result(results.CRITICAL, utils.ANegWG,
			"done", []results.ResultElem{arg1}, "", []results.ResultElem{})
		bugWasFound = true
	case utils.ExitCodeUnlockBeforeLock: // unlock of not locked mutex
		file, line, err := trace.PosFromPosString(exitPos)
		if err != nil {
			utils.LogError("Could not read exit pos: ", err)
		}
		arg1 := results.TraceElementResult{
			RoutineID: 0,
			ObjID:     0,
			TPre:      0,
			ObjType:   "ML",
			File:      file,
			Line:      line,
		}
		results.Result(results.CRITICAL, utils.AUnlockOfNotLockedMutex,
			"done", []results.ResultElem{arg1}, "", []results.ResultElem{})
		bugWasFound = true
	case utils.ExitCodePanic: // unknown panic
		file, line, err := trace.PosFromPosString(exitPos)
		if err != nil {
			utils.LogError("Could not read exit pos: ", err)
		}
		arg1 := results.TraceElementResult{
			RoutineID: 0,
			ObjID:     0,
			TPre:      0,
			ObjType:   "XX",
			File:      file,
			Line:      line,
		}
		results.Result(results.CRITICAL, utils.RUnknownPanic,
			"panic", []results.ResultElem{arg1}, "", []results.ResultElem{})
		bugWasFound = true
	case utils.ExitCodeTimeout: // timeout
		results.Result(results.CRITICAL, utils.RTimeout,
			"", []results.ResultElem{}, "", []results.ResultElem{})
	}

	if all {
		if exitCode == utils.ExitCodeSendClose { // send on closed
			file, line, err := trace.PosFromPosString(exitPos)
			if err != nil {
				utils.LogError("Could not read exit pos: ", err)
			}
			arg1 := results.TraceElementResult{ // send
				RoutineID: 0,
				ObjID:     0,
				TPre:      0,
				ObjType:   "CS",
				File:      file,
				Line:      line,
			}
			results.Result(results.CRITICAL, utils.ASendOnClosed,
				"send", []results.ResultElem{arg1}, "", []results.ResultElem{})
			bugWasFound = true
		}
	}
}

// RunHBAnalysis runs the full analysis happens before based analysis
//
// Parameter:
//   - assume_fifo bool: True to assume fifo ordering in buffered channels
//   - ignoreCriticalSections bool: True to ignore critical sections when updating vector clocks
//   - analysisCasesMap map[string]bool: The analysis cases to run
//   - fuzzing bool: true if run with fuzzing
//
// Returns:
//   - bool: true if something has been found
func RunHBAnalysis(assumeFifo bool, ignoreCriticalSections bool,
	analysisCasesMap map[string]bool, fuzzing bool) {
	fifo = assumeFifo
	modeIsFuzzing = fuzzing

	analysisCases = analysisCasesMap
	InitAnalysis(analysisCases, fuzzing)

	if analysisCases["resourceDeadlock"] {
		ResetState()
	}

	noRoutine := MainTrace.GetNoRoutines()
	for i := 1; i <= noRoutine; i++ {
		currentVC[i] = clock.NewVectorClock(noRoutine)
		currentWVC[i] = clock.NewVectorClock(noRoutine)
	}

	currentVC[1].Inc(1)
	currentWVC[1].Inc(1)

	utils.LogInfo("Start HB analysis")

	traceIter := MainTrace.AsIterator()

	for elem := traceIter.Next(); elem != nil; elem = traceIter.Next() {
		switch e := elem.(type) {
		case *trace.TraceElementAtomic:
			if ignoreCriticalSections {
				UpdateVCAtomicAlt(e)
			} else {
				UpdateVCAtomic(e)
			}
		case *trace.TraceElementChannel:
			UpdateVCChannel(e)
		case *trace.TraceElementMutex:
			if ignoreCriticalSections {
				UpdateVCMutexAlt(e)
			} else {
				UpdateVCMutex(e)
			}
			if analysisFuzzing {
				getConcurrentMutexForFuzzing(e)
			}
		case *trace.TraceElementFork:
			UpdateVCFork(e)
		case *trace.TraceElementSelect:
			cases := e.GetCases()
			ids := make([]int, 0)
			opTypes := make([]int, 0)
			for _, c := range cases {
				switch c.GetOpC() {
				case trace.SendOp:
					ids = append(ids, c.GetID())
					opTypes = append(opTypes, 0)
				case trace.RecvOp:
					ids = append(ids, c.GetID())
					opTypes = append(opTypes, 1)
				}
			}
			UpdateVCSelect(e)
		case *trace.TraceElementWait:
			UpdateVCWait(e)
		case *trace.TraceElementCond:
			UpdateVCCond(e)
		case *trace.TraceElementOnce:
			UpdateVCOnce(e)
			if analysisFuzzing {
				getConcurrentOnceForFuzzing(e)
			}
		case *trace.TraceElementRoutineEnd:
			UpdateVCRoutineEnd(e)
		case *trace.TraceElementNew:
			UpdateVCNew(e)
		}

		if analysisCases["resourceDeadlock"] {
			switch e := elem.(type) {
			case *trace.TraceElementMutex:
				HandleMutexEventForRessourceDeadlock(*e)
			}
		}

		// check for leak
		if analysisCases["leak"] && elem.GetTPost() == 0 {
			checkLeak(elem)
		}

		if memory.WasCanceled() {
			return
		}
	}

	MainTrace.SetHBWasCalc(true)

	utils.LogInfo("Finished HB analysis")

	if modeIsFuzzing {
		rerunCheckForSelectCaseWithPartnerChannel()
		CheckForSelectCaseWithPartner()
	}

	if memory.WasCanceled() {
		return
	}

	if analysisCases["leak"] {
		utils.LogInfo("Check for leak")
		checkForLeak()
		checkForStuckRoutine(false)
		utils.LogInfo("Finish check for leak")
	}

	if memory.WasCanceled() {
		return
	}

	if analysisCases["doneBeforeAdd"] {
		utils.LogInfo("Check for done before add")
		checkForDoneBeforeAdd()
		utils.LogInfo("Finish check for done before add")
	}

	if memory.WasCanceled() {
		return
	}

	// if memory.WasCanceled() {
	// 	return
	// }

	if analysisCases["resourceDeadlock"] {
		utils.LogInfo("Check for cyclic deadlock")
		CheckForResourceDeadlock()
		utils.LogInfo("Finish check for cyclic deadlock")
	}

	if memory.WasCanceled() {
		return
	}

	if analysisCases["unlockBeforeLock"] {
		utils.LogInfo("Check for unlock before lock")
		checkForUnlockBeforeLock()
		utils.LogInfo("Finish check for unlock before lock")
	}
}

// checkLeak checks for a given element if it leaked (has no tPost). If so,
// it will look for a possible way to resolve the leak
//
// Parameter:
//   - elem TraceElement: Element to check
func checkLeak(elem trace.TraceElement) {
	switch e := elem.(type) {
	case *trace.TraceElementChannel:
		CheckForLeakChannelStuck(e, currentVC[e.GetRoutine()])
	case *trace.TraceElementMutex:
		CheckForLeakMutex(e)
	case *trace.TraceElementWait:
		CheckForLeakWait(e)
	case *trace.TraceElementSelect:
		timer.Start(timer.AnaLeak)
		cases := e.GetCases()
		ids := make([]int, 0)
		buffered := make([]bool, 0)
		opTypes := make([]int, 0)
		for _, c := range cases {
			switch c.GetOpC() {
			case trace.SendOp:
				ids = append(ids, c.GetID())
				opTypes = append(opTypes, 0)
				buffered = append(buffered, c.IsBuffered())
			case trace.RecvOp:
				ids = append(ids, c.GetID())
				opTypes = append(opTypes, 1)
				buffered = append(buffered, c.IsBuffered())
			}
		}
		timer.Stop(timer.AnaLeak)
		CheckForLeakSelectStuck(e, ids, buffered, currentVC[e.GetRoutine()], opTypes)
	case *trace.TraceElementCond:
		CheckForLeakCond(e)
	}
}
