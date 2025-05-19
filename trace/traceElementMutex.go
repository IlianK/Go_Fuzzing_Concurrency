// Copyright (c) 2024 Erik Kassubek
//
// File: traceElementMutex.go
// Brief: Struct and functions for mutex operations in the trace
//
// Author: Erik Kassubek
// Created: 2023-08-08
//
// License: BSD-3-Clause

package trace

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"advocate/clock"
)

// OpMutex is an enum for opM
type OpMutex int

// Values for the opMutex enum
const (
	LockOp OpMutex = iota
	RLockOp
	TryLockOp
	TryRLockOp
	UnlockOp
	RUnlockOp
)

// TraceElementMutex is a trace element for a mutex
//
// Fields:
//   - traceID: id of the element, should never be changed
//   - index int: Index in the routine
//   - routine int: The routine id
//   - tPre int: The timestamp at the start of the event
//   - tPost int: The timestamp at the end of the event
//   - id int: The id of the mutex
//   - rw bool: Whether the mutex is a read-noWarningrite mutex
//   - opM opMutex: The operation on the mutex
//   - suc bool: Whether the operation was successful (only for trylock else always true)
//   - file string: The file of the mutex operation in the code
//   - line int: The line of the mutex operation in the code
//   - vc *clock.VectorClock: The vector clock of the operation
//   - wVc *clock.VectorClock: The weak vector clock of the operation
type TraceElementMutex struct {
	traceID int
	index   int
	routine int
	tPre    int
	tPost   int
	id      int
	rw      bool
	opM     OpMutex
	suc     bool
	file    string
	line    int
	vc      *clock.VectorClock
	wVc     *clock.VectorClock
}

// AddTraceElementMutex adds a new mutex element to the main trace
//
// Parameter:
//   - routine int: The routine id
//   - tPre string: The timestamp at the start of the event
//   - tPost string: The timestamp at the end of the event
//   - id string: The id of the mutex
//   - rw string: Whether the mutex is a read-noWarningrite mutex
//   - opM string: The operation on the mutex
//   - suc string: Whether the operation was successful (only for trylock else always true)
//   - pos string: The position of the mutex operation in the code
func (t *Trace) AddTraceElementMutex(routine int, tPre string,
	tPost string, id string, rw string, opM string, suc string,
	pos string) error {
	tPreInt, err := strconv.Atoi(tPre)
	if err != nil {
		return errors.New("tPre is not an integer")
	}

	tPostInt, err := strconv.Atoi(tPost)
	if err != nil {
		return errors.New("tPost is not an integer")
	}

	idInt, err := strconv.Atoi(id)
	if err != nil {
		return errors.New("id is not an integer")
	}

	rwBool := false
	if rw == "t" {
		rwBool = true
	}

	var opMInt OpMutex
	switch opM {
	case "L":
		opMInt = LockOp
	case "R":
		opMInt = RLockOp
	case "T":
		opMInt = TryLockOp
	case "Y":
		opMInt = TryRLockOp
	case "U":
		opMInt = UnlockOp
	case "N":
		opMInt = RUnlockOp
	default:
		return errors.New("opM is not a valid operation")
	}

	sucBool, err := strconv.ParseBool(suc)
	if err != nil {
		return errors.New("suc is not a boolean")
	}

	file, line, err := PosFromPosString(pos)
	if err != nil {
		return err
	}

	elem := TraceElementMutex{
		index:   t.numberElemsInTrace[routine],
		routine: routine,
		tPre:    tPreInt,
		tPost:   tPostInt,
		id:      idInt,
		rw:      rwBool,
		opM:     opMInt,
		suc:     sucBool,
		file:    file,
		line:    line,
		vc:      nil,
		wVc:     nil,
	}

	t.AddElement(&elem)
	return nil
}

// GetID returns the ID of the primitive on which the operation was executed
//
// Returns:
//   - int: The id of the element
func (mu *TraceElementMutex) GetID() int {
	return mu.id
}

// GetRoutine returns the routine ID of the element.
//
// Returns:
//   - int: The routine of the element
func (mu *TraceElementMutex) GetRoutine() int {
	return mu.routine
}

// GetTPre returns the tPre of the element.
//
// Returns:
//   - int: The tPre of the element
func (mu *TraceElementMutex) GetTPre() int {
	return mu.tPre
}

// GetTPost returns the tPost of the element.
//
// Returns:
//   - int: The tPost of the element
func (mu *TraceElementMutex) GetTPost() int {
	return mu.tPost
}

// GetTSort returns the timer value, that is used for the sorting of the trace
//
// Returns:
//   - int: The timer of the element
func (mu *TraceElementMutex) GetTSort() int {
	if mu.tPost == 0 {
		// add at the end of the trace
		return math.MaxInt
	}
	return mu.tPost
}

// GetPos returns the position of the operation in the form [file]:[line].
//
// Returns:
//   - string: The position of the element
func (mu *TraceElementMutex) GetPos() string {
	return fmt.Sprintf("%s:%d", mu.file, mu.line)
}

// GetReplayID returns the replay id of the element
//
// Returns:
//   - The replay id
func (mu *TraceElementMutex) GetReplayID() string {
	return fmt.Sprintf("%d:%s:%d", mu.routine, mu.file, mu.line)
}

// GetFile returns the file where the operation represented by the element was executed
//
// Returns:
//   - The file of the element
func (mu *TraceElementMutex) GetFile() string {
	return mu.file
}

// GetLine returns the line where the operation represented by the element was executed
//
// Returns:
//   - The line of the element
func (mu *TraceElementMutex) GetLine() int {
	return mu.line
}

// GetTID returns the tID of the element.
// The tID is a string of form [file]:[line]@[tPre]
//
// Returns:
//   - string: The tID of the element
func (mu *TraceElementMutex) GetTID() string {
	return mu.GetPos() + "@" + strconv.Itoa(mu.tPre)
}

// GetOpM returns the operation
//
// Returns:
//   - OpMutex: the operation
func (mu *TraceElementMutex) GetOpM() OpMutex {
	return mu.opM
}

// GetOperation returns the operation of the element
//
// Returns:
//   - OpMutex: The operation of the element
func (mu *TraceElementMutex) GetOperation() OpMutex {
	return mu.opM
}

// IsLock returns if the element is a lock operation
//
// Returns:
//   - bool: If the element is a lock operation
func (mu *TraceElementMutex) IsLock() bool {
	return mu.opM == LockOp || mu.opM == RLockOp || mu.opM == TryLockOp || mu.opM == TryRLockOp
}

// SetVc sets the vector clock
//
// Parameter:
//   - vc *clock.VectorClock: the vector clock
func (mu *TraceElementMutex) SetVc(vc *clock.VectorClock) {
	mu.vc = vc.Copy()
}

// SetWVc sets the weak vector clock
//
// Parameter:
//   - vc *clock.VectorClock: the vector clock
func (mu *TraceElementMutex) SetWVc(vc *clock.VectorClock) {
	mu.wVc = vc.Copy()
}

// GetVC returns the vector clock of the element
//
// Returns:
//   - VectorClock: The vector clock of the element
func (mu *TraceElementMutex) GetVC() *clock.VectorClock {
	return mu.vc
}

// GetWVc returns the weak vector clock of the element
//
// Returns:
//   - VectorClock: The vector clock of the element
func (mu *TraceElementMutex) GetWVc() *clock.VectorClock {
	return mu.wVc
}

// GetObjType returns the string representation of the object type
//
// Parameter:
//   - operation bool: if true get the operation code, otherwise only the primitive code
//
// Returns:
//   - string: the object type
func (mu *TraceElementMutex) GetObjType(operation bool) string {
	if !operation {
		return ObjectTypeMutex
	}

	switch mu.opM {
	case LockOp:
		return ObjectTypeMutex + "L"
	case RLockOp:
		return ObjectTypeMutex + "R"
	case TryLockOp:
		return ObjectTypeMutex + "T"
	case TryRLockOp:
		return ObjectTypeMutex + "Y"
	case UnlockOp:
		return ObjectTypeMutex + "U"
	case RUnlockOp:
		return ObjectTypeMutex + "N"
	}
	return ObjectTypeMutex
}

// IsSuc returns whether the locking was successful of the element
//
// Returns:
//   - For trylock wether it was successful, otherwise always true
func (mu *TraceElementMutex) IsSuc() bool {
	return mu.suc
}

// IsEqual checks if an trace element is equal to this element
//
// Parameter:
//   - elem TraceElement: The element to check against
//
// Returns:
//   - bool: true if it is the same operation, false otherwise
func (mu *TraceElementMutex) IsEqual(elem TraceElement) bool {
	return mu.routine == elem.GetRoutine() && mu.ToString() == elem.ToString()
}

// GetTraceIndex returns trace local index of the element in the trace
//
// Returns:
//   - int: the routine id of the element
//   - int: The trace local index of the element in the trace
func (mu *TraceElementMutex) GetTraceIndex() (int, int) {
	return mu.routine, mu.index
}

// SetT sets the tPre and tPost of the element
//
// Parameter:
//   - time int: The tPre and tPost of the element
func (mu *TraceElementMutex) SetT(time int) {
	mu.tPre = time
	mu.tPost = time
}

// SetTPre sets the tPre of the element.
//
// Parameter:
//   - tPre int: The tPre of the element
func (mu *TraceElementMutex) SetTPre(tPre int) {
	mu.tPre = tPre
	if mu.tPost != 0 && mu.tPost < tPre {
		mu.tPost = tPre
	}
}

// SetTSort sets the timer, that is used for the sorting of the trace
//
// Parameter:
//   - tSort int: The timer of the element
func (mu *TraceElementMutex) SetTSort(tSort int) {
	mu.SetTPre(tSort)
	mu.tPost = tSort
}

// SetTWithoutNotExecuted set the timer, that is used for the sorting of the trace, only if the original
// value was not 0
//
// Parameter:
//   - tSort int: The timer of the element
func (mu *TraceElementMutex) SetTWithoutNotExecuted(tSort int) {
	mu.SetTPre(tSort)
	if mu.tPost != 0 {
		mu.tPost = tSort
	}
}

// ToString returns the simple string representation of the element
//
// Returns:
//   - string: The simple string representation of the element
func (mu *TraceElementMutex) ToString() string {
	res := "M,"
	res += strconv.Itoa(mu.tPre) + "," + strconv.Itoa(mu.tPost) + ","
	res += strconv.Itoa(mu.id) + ","

	if mu.rw {
		res += "R,"
	} else {
		res += "-,"
	}

	switch mu.opM {
	case LockOp:
		res += "L"
	case RLockOp:
		res += "R"
	case TryLockOp:
		res += "T"
	case TryRLockOp:
		res += "Y"
	case UnlockOp:
		res += "U"
	case RUnlockOp:
		res += "N"
	}

	if mu.suc {
		res += ",t"
	} else {
		res += ",f"
	}
	res += "," + mu.GetPos()
	return res
}

// GetTraceID returns the trace id
//
// Returns:
//   - int: the trace id
func (mu *TraceElementMutex) GetTraceID() int {
	return mu.traceID
}

// GetTraceID sets the trace id
//
// Parameter:
//   - ID int: the trace id
func (mu *TraceElementMutex) setTraceID(ID int) {
	mu.traceID = ID
}

// Copy the element
//
// Returns:
//   - TraceElement: The copy of the element
func (mu *TraceElementMutex) Copy() TraceElement {
	return &TraceElementMutex{
		traceID: mu.traceID,
		index:   mu.index,
		routine: mu.routine,
		tPre:    mu.tPre,
		tPost:   mu.tPost,
		id:      mu.id,
		rw:      mu.rw,
		opM:     mu.opM,
		suc:     mu.suc,
		file:    mu.file,
		line:    mu.line,
		vc:      mu.vc.Copy(),
		wVc:     mu.wVc.Copy(),
	}
}
