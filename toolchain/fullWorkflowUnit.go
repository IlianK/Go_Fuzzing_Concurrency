// Copyright (c) 2024 Erik Kassubek, Mario Occhinegro
//
// File: runFullWorkflowMain.go
// Brief: Function to run the whole ADVOCATE workflow, including running,
//    analysis and replay on all unit tests of a program
//
// Author: Erik Kassubek, Mario Occhinegro
// Created: 2024-09-18
//
// License: BSD-3-Clause

package toolchain

import (
	"advocate/analysis"
	"advocate/complete"
	"advocate/memory"
	"advocate/results"
	"advocate/stats"
	"advocate/timer"
	"advocate/utils"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Run ADVOCATE for all given unit tests
//
// Parameter:
//   - pathToAdvocate string: pathToAdvocate
//   - dir string: path to the folder containing the unit tests
//   - runRecord bool: run the recording. If set to false, but runAnalysis or runReplay is
//     set the trace at tracePath is used
//   - runAnalysis bool: run the analysis on a path
//   - runReplay bool: run replay, if runAnalysis is true, those replays are used
//   - pathToTest string: path to the test file, should be set if exec is set
//   - progName string: name of the analyzed program
//   - measureTime bool: if true, measure the time for all steps. This
//   - also runs the tests once without any recoding/replay to get a base value
//   - notExecuted bool: if true, check for never executed operations
//   - createStats bool: create a stats file
//   - fuzzing int: -1 if not fuzzing, otherwise number of fuzzing run, starting with 0
//   - fuzzingTrace string: path to the fuzzing trace path. If not used path (GFuzz or Flow), opr not fuzzing, set to empty string
//   - keepTraces bool: do not delete traces after analysis
//   - firstRun bool: this is the first run, only set to false for fuzzing (except for the first fuzzing)
//   - skipExisting bool: do not overwrite existing results, skip those tests
//   - cont bool: continue an already started run
//   - onlyRecord bool: if true, run only the recording without any analysis
//
// Returns:
//   - error
func runWorkflowUnit(pathToAdvocate, dir string, runRecord, runAnalysis, runReplay bool,
	pathToTest, progName string,
	notExecuted, createStats bool, fuzzing int, fuzzingTrace string,
	keepTraces, firstRun, skipExisting, cont bool, fileNumber,
	testNumber int) error {
	// Validate required inputs
	if pathToAdvocate == "" {
		return errors.New("Path to advocate is empty")
	}
	if dir == "" {
		return errors.New("Directory is empty")
	}

	isFuzzing := (fuzzing != -1)

	// Change to the directory
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("Failed to change directory: %v", dir)
	}

	if firstRun && !cont {
		if !skipExisting {
			os.RemoveAll("advocateResult")
		}

		if info, _ := os.Stat("advocateResult"); info == nil {
			if err := os.MkdirAll("advocateResult", os.ModePerm); err != nil {
				return fmt.Errorf("Failed to create advocateResult directory: %v", err)
			}
		}

		// Remove possibly leftover traces from unexpected aborts that could interfere with replay
		removeTraces(dir)
		removeLogs(dir)
	}

	// Find all _test.go files in the directory
	testFiles, _, totalFiles, err := FindTestFiles(dir, cont && testName == "")
	if err != nil {
		return fmt.Errorf("Failed to find test files: %v", err)
	}

	attemptedTests, skippedTests, currentFile := 0, 0, fileNumber

	// resultPath := filepath.Join(dir, "advocateResult")

	ranTest := false
	// Process each test file
	for _, file := range testFiles {
		if testName == "" {
			utils.LogProgressf("Progress %s: %d/%d", progName, currentFile, totalFiles)
			utils.LogProgressf("Processing file: %s", file)
		}

		packagePath := filepath.Dir(file)
		testFunctions, err := FindTestFunctions(file)
		if err != nil {
			utils.LogInfof("Could not find test functions in %s: %v", file, err)
			continue
		}

		for _, testFunc := range testFunctions {
			if (pathToTest == "" || pathToTest != file) && testName != "" && testName != testFunc {
				continue
			}

			analysis.Clear()
			memory.Reset()

			if !isFuzzing {
				timer.ResetTest()
				timer.Start(timer.TotalTest)
			}

			ranTest = true

			attemptedTests++
			packageName := filepath.Base(packagePath)
			fileName := filepath.Base(file)

			if fuzzing == -1 {
				utils.LogProgressf("Running test %s in package %s in file %s", testFunc, packageName, file)
			}

			adjustedPackagePath := strings.TrimPrefix(packagePath, dir)
			if !strings.HasSuffix(adjustedPackagePath, string(filepath.Separator)) {
				adjustedPackagePath = adjustedPackagePath + string(filepath.Separator)
			}
			fileNameWithoutEnding := strings.TrimSuffix(fileName, ".go")
			directoryName := filepath.Join("advocateResult", fmt.Sprintf("file(%d)-test(%d)-%s-%s", currentFile, attemptedTests, fileNameWithoutEnding, testFunc))
			if cont && fileNumber != 0 {
				directoryName = filepath.Join("advocateResult", fmt.Sprintf("file(%d)-test(%d)-%s-%s", fileNumber, testNumber, fileNameWithoutEnding, testFunc))
			}
			currentResFolder = filepath.Join(dir, directoryName)

			if fuzzing < 1 {
				utils.LogInfo("Create ", directoryName)
				if err := os.MkdirAll(directoryName, os.ModePerm); err != nil {
					utils.LogErrorf("Failed to create directory %s: %v", directoryName, err)
					if !isFuzzing {
						timer.Stop(timer.TotalTest)
					}
					continue
				}
			}

			// Execute full workflow
			nrReplay, anaPassed, err := unitTestFullWorkflow(pathToAdvocate,
				dir, runRecord, runAnalysis, runReplay, testFunc, adjustedPackagePath, file, fuzzing,
				fuzzingTrace)

			timer.UpdateTimeFileDetail(progName, testFunc, nrReplay)

			if !isFuzzing {
				timer.ResetTest()
				timer.UpdateTimeFileOverview(progName, testFunc)
			}

			// Move logs and results to the appropriate directory
			total := fuzzing != -1
			collect(dir, packagePath, currentResFolder, total)

			if err != nil {
				utils.LogErrorf(err.Error())
				skippedTests++
			}

			if anaPassed {
				generateBugReports(currentResFolder, fuzzing)
				if createStats {
					// create statistics
					err := stats.CreateStats(currentResFolder, progName, testFunc, movedTraces, fuzzing)
					if err != nil {
						utils.LogError("Could not create statistics: ", err.Error())
					}
				}
			}

			if !keepTraces {
				removeTraces(dir)
			}

			if total {
				removeLogs(dir)
			}

			if !isFuzzing {
				timer.Stop(timer.TotalTest)
			}
		}

		currentFile++
	}

	if testName != "" && !ranTest {
		return fmt.Errorf("could not find test function %s", testName)
	}

	// Check for untriggered selects
	if notExecuted && testName != "" {
		err := complete.Check(filepath.Join(dir, "advocateResult"), dir)
		if err != nil {
			fmt.Println("Could not run check for untriggered select and not executed progs")
		}
	}

	// Output test summary
	if testName == "" {
		utils.LogInfo("Finished full workflow for all tests")
		utils.LogInfof("Attempted tests: %d", attemptedTests)
		utils.LogInfof("Skipped tests: %d", skippedTests)
	} else {
		utils.LogInfof("Finished full work flow for %s", testName)
	}

	return nil
}

// FindTestFiles finds all _test.go files in the specified directory
//
// Parameter:
//   - dir string: folder to search in
//   - cont bool: only return test files not already in the advocateResult
//
// Returns:
//   - []string: found files
//   - int: min file num, only if cont, otherwise 0
//   - int: total number of files
//   - error
func FindTestFiles(dir string, cont bool) ([]string, int, int, error) {
	var testFiles []string

	alreadyProcessed, maxFileNum := make(map[string]struct{}), 0
	var err error

	if cont {
		alreadyProcessed, maxFileNum, err = getFilesInResult(dir, cont)
		if err != nil {
			utils.LogError(err)
			return testFiles, 0, 0, err
		}
	}

	totalNumFiles := 0
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if strings.HasSuffix(name, "_test.go") {
			totalNumFiles++
			if _, ok := alreadyProcessed[name]; !cont || !ok {
				testFiles = append(testFiles, path)
			}
		}
		return nil
	})
	if err != nil {
		utils.LogError(err)
	}
	return testFiles, maxFileNum, totalNumFiles, err
}

// getFilesInResult finds all test files that have already been run and therefore
// have an entry in the results. Clear up incomplete result folders
//
// Parameter:
//   - dir string: path to the directory containing the test files
//   - cont bool: if true, exclude the last executed test file and remove its result file
//
// Returns:
//   - map[string]struct{}: map containing already processed test files
//   - int: total number of files
//   - error
func getFilesInResult(dir string, cont bool) (map[string]struct{}, int, error) {
	res := make(map[string]struct{})

	path := filepath.Join(dir, "advocateResult")

	patternPrefix := `file\([0-9]+\)-test\([0-9]+\)-`
	patternFileNum := `^file\((\d+)\)-test\(\d+\)-.+$`
	rePrefix := regexp.MustCompile(patternPrefix)
	reNum := regexp.MustCompile(patternFileNum)

	files, err := os.ReadDir(path)
	if err != nil {
		return res, 0, err
	}

	maxFileNum := -1
	maxKey := ""
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		name := file.Name()
		nameClean := rePrefix.ReplaceAllString(name, "")
		lastIndex := strings.LastIndex(nameClean, "-")
		if lastIndex != -1 {
			nameClean = nameClean[:lastIndex] // Keep everything before the last separator
		}

		numbers := reNum.FindStringSubmatch(name)

		if len(numbers) > 1 {
			numberInt, err := strconv.Atoi(numbers[1])
			if err != nil {
				return res, 0, err
			}
			if numberInt > maxFileNum {
				maxKey = nameClean + ".go"
				maxFileNum = numberInt
			}
		}

		res[nameClean+".go"] = struct{}{}
	}

	// remove all folders created by the last file and remove the file name from the processed
	if cont && maxFileNum != -1 {
		for _, file := range files {
			if !file.IsDir() || !strings.Contains(file.Name(), fmt.Sprintf("file(%d)", maxFileNum)) {
				continue
			}

			_ = os.RemoveAll(filepath.Join(path, file.Name()))
		}
		utils.LogError()
		delete(res, maxKey)
		maxFileNum = maxFileNum - 1
	}

	return res, maxFileNum, nil
}

// FindTestFunctions find all test function in the specified file
//
// Parameter:
//   - file string: file to search in
//
// Returns:
//   - []string: functions
//   - error
func FindTestFunctions(file string) ([]string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var testFunctions []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "func Test") && strings.Contains(line, "*testing.T") {
			testFunc := strings.TrimSpace(strings.Split(line, "(")[0])
			testFunc = strings.TrimPrefix(testFunc, "func ")
			testFunctions = append(testFunctions, testFunc)
		}
	}
	return testFunctions, nil
}

// Run the full workflow for a given unit test.
// This will run, record, analyzer and, if necessary, rewrite and replay the test
//
// Parameter:
//   - pathToAdvocate string: path to advocate
//   - dir string: path to the package to test
//   - runRecord bool: run the recording. If set to false, but runAnalysis or runReplay is
//     set the trace at tracePath is used
//   - runAnalysis bool: run the analysis on a path
//   - runReplay bool: run replay, if runAnalysis is true, those replays are used
//   - progName string: name of the program
//   - testName string: name of the test
//   - pkg string: adjusted package path
//   - file string: file with the test
//   - fuzzing int: -1 if not fuzzing, otherwise number of fuzzing run, starting with 0
//   - fuzzingTrace string: the path to the fuzzing trace
//   - onlyRecord bool: if true, run only the recording without any analysis
//
// Returns:
//   - int: number of run replays
//   - bool: true if analysis passed without error
//   - error
func unitTestFullWorkflow(pathToAdvocate, dir string,
	runRecord, runAnalysis, runReplay bool,
	testName, pkg, file string,
	fuzzing int, fuzzingTrace string) (int, bool, error) {
	output := "output.log"

	outFile, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, false, fmt.Errorf("Failed to open log file: %v", err)
	}
	defer outFile.Close()

	// Redirect stdout and stderr to the file
	origStdout := os.Stdout
	origStderr := os.Stderr

	os.Stdout = outFile
	os.Stderr = outFile

	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	// Validate required inputs
	if pathToAdvocate == "" {
		return 0, false, errors.New("Path to advocate is empty")
	}
	if dir == "" {
		return 0, false, errors.New("Directory is empty")
	}
	if testName == "" {
		return 0, false, errors.New("Test name is empty")
	}
	// if pkg == "" {
	// 	return 0, errors.New("Package is empty")
	// }
	if file == "" {
		return 0, false, errors.New("Test file is empty")
	}

	pathToPatchedGoRuntime := filepath.Join(pathToAdvocate, "go-patch/bin/go")
	pathToGoRoot := filepath.Join(pathToAdvocate, "go-patch")

	if runtime.GOOS == "windows" {
		pathToPatchedGoRuntime += ".exe"
	}

	// Change to the directory
	if err := os.Chdir(dir); err != nil {
		return 0, false, fmt.Errorf("Failed to change directory: %v", err)
	}

	pkg = strings.TrimPrefix(pkg, dir)

	if runRecord {
		if measureTime && fuzzing < 1 {
			err := unitTestRun(pkg, file, testName, origStdout, origStderr)
			if err != nil {
				if checkForTimeout(output) {
					utils.LogTimeout("Running T0 timed out")
				}
			}
		}

		err = unitTestRecord(pathToGoRoot, pathToPatchedGoRuntime, pkg, file,
			testName, fuzzing, fuzzingTrace, output, origStdout, origStderr)
		if err != nil {
			utils.LogError("Recording failed: ", err.Error())
		}
	}

	if runAnalysis {
		pkgPath := filepath.Join(dir, pkg)
		err = unitTestAnalyzer(pkgPath, "advocateTrace", fuzzing)
		if err != nil {
			return 0, false, err
		}

		if onlyAPanicAndLeakFlag {
			return 0, true, nil
		}
	}

	numberReplay := 0
	if runReplay {
		numberReplay = unitTestReplay(pathToGoRoot, pathToPatchedGoRuntime, dir, pkg, file, testName, output, runAnalysis, origStdout, origStderr)
	}

	return numberReplay, true, nil
}

// unitTestRun runs a test without recording/replay
//
// Parameter:
//   - pkg string: path to the package containing the test
//   - file string: path to the file containing the test function
//   - name of the test function to run
//   - osOut *os.File: file/output to write to not being what os.Stdout points to
//   - osErr *os.File: file/output to write to not being what os.Stdout points to
//
// Returns:
//   - error
func unitTestRun(pkg, file, testName string, origStdout, origStderr *os.File) error {
	timer.Start(timer.Run)
	defer timer.Stop(timer.Run)

	// Remove header just in case
	if err := headerRemoverUnit(file); err != nil {
		utils.LogError("Failed to remove header: ", err)
	}

	os.Unsetenv("GOROOT")

	utils.LogInfo("Run T0")
	packagePath := utils.MakePathLocal(pkg)
	var err error
	if timeoutRecording != -1 {
		timeoutRecString := fmt.Sprintf("%ds", timeoutRecording)
		err = runCommand(origStdout, origStderr, "go", "test", "-v", "-timeout", timeoutRecString, "-count=1", "-run="+testName, packagePath)
	} else {
		err = runCommand(origStdout, origStderr, "go", "test", "-v", "-count=1", "-run="+testName, packagePath)
	}

	return err
}

// unitTestRun runs a test and records the trace and run info
//
// Parameter:
//   - pathToFoRoot string: path to the root of the modified go runtime
//   - pathToPatchedGoRuntime string: path to the patched runtime executable
//   - pkg string: path to the package containing the test
//   - file string: path to the file containing the test function
//   - testName string: name of the test function to run
//   - fuzzing int: number of fuzzing run. If not fuzzing, or first fuzzing run without guidance set to 0
//   - fuzzingPath string: path to the fuzzing guidance information file, if not fuzzing set to ""
//   - output string: path to where the output should be created
//   - osOut *os.File: file/output to write to not being what os.Stdout points to
//   - osErr *os.File: file/output to write to not being what os.Stdout points to
//
// Returns:
//   - error
func unitTestRecord(pathToGoRoot, pathToPatchedGoRuntime, pkg, file, testName string,
	fuzzing int, fuzzingPath, output string, osOut, osErr *os.File) error {
	timer.Start(timer.Recording)
	defer timer.Stop(timer.Recording)

	isFuzzing := (fuzzing > 0)

	// Remove header just in case
	if err := headerRemoverUnit(file); err != nil {
		return fmt.Errorf("Failed to remove header: %v", err)
	}

	// Add header
	if err := headerInserterUnit(file, testName, false, fuzzing, fuzzingPath, timeoutReplay, false); err != nil {
		return fmt.Errorf("Error in adding header: %v", err)
	}

	// Run the test
	utils.LogInfo("Execute Test")

	// Set GOROOT
	os.Setenv("GOROOT", pathToGoRoot)

	runCommand(osOut, osErr, pathToPatchedGoRuntime, "version")

	pkgPath := utils.MakePathLocal(pkg)
	err := runCommand(osOut, osErr, pathToPatchedGoRuntime, "test", "-gcflags=all=-N -l", "-v", "-count=1", "-run="+testName, pkgPath)
	if err != nil {
		if isFuzzing {
			if checkForTimeout(output) {
				utils.LogTimeout("Recording timed out")
			}
		} else {
			if checkForTimeout(output) {
				utils.LogTimeout("Fuzzing recording timed out")
			}
		}
	}

	err = os.Unsetenv("GOROOT")

	if err != nil {
		utils.LogErrorf("Failed to unset GOROOT: ", err.Error())
	}

	// Remove header after the test
	err = headerRemoverUnit(file)

	return err
}

// unitTestRun runs the analysis on a recorded trace
//
// Parameter:
//   - pkgPath string: path to the analyzed package
//   - traceName string: name of the trace to analyze
//   - fuzzing int: number of fuzzing run. If not fuzzing, or first fuzzing run without guidance set to 0
//
// Returns:
//   - error
//
// The trace is expected to be at dir/pkg/traceName
func unitTestAnalyzer(pkgPath, traceName string, fuzzing int) error {
	tracePath := filepath.Join(pkgPath, traceName)

	utils.LogInfof("Run the analyzer for %s", tracePath)

	outM := filepath.Join(pkgPath, "results_machine.log")
	outR := filepath.Join(pkgPath, "results_readable.log")
	outT := filepath.Join(pkgPath, "rewrittenTrace")
	err := runAnalyzer(tracePath, noRewriteFlag, analysisCasesFlag, outR,
		outM, ignoreAtomicsFlag, fifoFlag, ignoreCriticalSectionFlag, rewriteAllFlag,
		outT, fuzzing, onlyAPanicAndLeakFlag)

	if err != nil {
		return err
	}

	return nil
}

// unitTestReplay runs a replay for a test
//
// Parameter:
//   - pathToFoRoot string: path to the root of the modified go runtime
//   - pathToPatchedGoRuntime string: path to the patched runtime executable
//   - dir: path to the root of the analyzed project
//   - pkg string: path to the package containing the test, global path should be dir/pkg
//   - file string: path to the file containing the test function
//   - testName string: name of the test function to run
//   - output string: path to the output file
//   - runAnalysis bool: whether the rewritten traces from the analysis or the
//     given trace path should be used
//   - osOut *os.File: file/output to write to not being what os.Stdout points to
//   - osErr *os.File: file/output to write to not being what os.Stdout points to
//
// Returns:
//   - int: number of executed replays
func unitTestReplay(pathToGoRoot, pathToPatchedGoRuntime, dir, pkg, file,
	testName, output string, fromAnalysis bool, osOut, osErr *os.File) int {
	timer.Start(timer.Replay)
	defer timer.Stop(timer.Replay)

	utils.LogInfo("Start Replay")

	pathPkg := filepath.Join(dir, pkg)

	rewrittenTraces := make([]string, 0)

	if fromAnalysis {
		rewrittenTraces, _ = filepath.Glob(filepath.Join(pathPkg, "rewrittenTrace_*"))
	} else {
		rewrittenTraces = append(rewrittenTraces, tracePathFlag)
	}

	utils.LogInfof("Found %d rewritten traces", len(rewrittenTraces))

	for i, trace := range rewrittenTraces {
		traceNum, bugString := extractTraceNumber(trace)
		// record := getRerecord(trace)
		record := false

		// we do not need to replay a bug that has already been replayed by
		// another replay
		if !replayAllFlag && results.WasAlreadyConfirmed(bugString) {
			// TODO: check if the report are still working with this
			continue
		}

		headerInserterUnit(file, testName, true, -1, traceNum, timeoutReplay, record)

		os.Setenv("GOROOT", pathToGoRoot)

		utils.LogInfof("Run replay %d/%d", i+1, len(rewrittenTraces))
		pkgPath := utils.MakePathLocal(pkg)
		runCommand(osOut, osErr, pathToPatchedGoRuntime, "test", "-gcflags=all=-N -l", "-v", "-count=1", "-run="+testName, pkgPath)
		utils.LogInfof("Finished replay %d/%d", i+1, len(rewrittenTraces))

		if wasReplaySuc(output) {
			results.AddBug(bugString, true)
		} else {
			results.AddBug(bugString, false)
		}

		os.Unsetenv("GOROOT")

		// Remove reorder header
		headerRemoverUnit(file)
	}

	return len(rewrittenTraces)
}
