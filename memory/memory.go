// Copyright (c) 2025 Erik Kassubek
//
// File: memory.go
// Brief: Cancel analysis when not enough memory
//
// Author: Erik Kassubek
// Created: 2025-03-03
//
// License: BSD-3-Clause

package memory

import (
	"advocate/utils"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/mem"
)

var (
	wasCanceled    atomic.Bool
	wasCanceledRAM atomic.Bool
)

// Supervisor periodically checks the used and free memory
// If the trace is to big and the available RAM to small, this can lead
// to problems. In this case we abort the analysis
func Supervisor() {
	// Get the memory stats
	v, err := mem.VirtualMemory()
	if err != nil {
		utils.LogErrorf("Error getting memory info: %v", err)
	}

	// Get the swap stats
	s, err := mem.SwapMemory()
	if err != nil {
		utils.LogErrorf("Error getting swap info: %v", err)
	}

	thresholdRAM := uint64(float64(v.Total) * 0.02)
	thresholdSwap := uint64(10 * 1024 * 1024) // 1MB

	startSwap := s.Used

	for {
		// Get the memory stats
		v, err = mem.VirtualMemory()
		if err != nil {
			utils.LogErrorf("Error getting memory info: %v", err)
		}

		// Get the swap stats
		s, err = mem.SwapMemory()
		if err != nil {
			utils.LogErrorf("Error getting swap info: %v", err)
		}

		// cancel if available RAM is below the threshold or the used swap is above the threshhold
		if v.Available < thresholdRAM {
			cancelRAM()
			return
		}

		if s.Used > thresholdSwap+startSwap {
			cancelRAM()
			return
		}

		// Sleep for a while before checking again
		time.Sleep(500 * time.Millisecond)
	}
}

// Cancel sets the analysis to canceled
func Cancel() {
	wasCanceled.Store(true)
}

// Cancel the analysis if not enough ram is available
func cancelRAM() {
	wasCanceled.Store(true)
	wasCanceledRAM.Store(true)
	printAllGoroutines()
	utils.LogError("Not enough RAM")
}

// WasCanceled returns if the analysis was canceled
//
// Returns:
//   - bool: true if the analysis was canceled
func WasCanceled() bool {
	return wasCanceled.Load()
}

// WasCanceledRAM returns if the analysis was canceled because of insufficient ram
//
// Returns:
//   - bool: true if the analysis was canceled because of insufficient ram*
func WasCanceledRAM() bool {
	return wasCanceledRAM.Load()
}

// Reset the cancel values to false
func Reset() {
	wasCanceled.Store(false)
	wasCanceledRAM.Store(false)
}
