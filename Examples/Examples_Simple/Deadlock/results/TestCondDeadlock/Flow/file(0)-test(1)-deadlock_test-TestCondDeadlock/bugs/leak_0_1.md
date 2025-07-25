# Leak: Leak on sync.Cond

The analyzer detected a leak on a sync.Cond.
A leak on a sync.Cond is a situation, where a sync.Cond wait is still blocking at the end of the program.
A sync.Cond wait is blocking, because the condition is not met.

## Test/Program
The bug was found in the following test/program:

- Test/Prog: TestCondDeadlock
- File: /home/ilian/Projects/Go_Fuzzing_Concurrency/Examples/Examples_Own/Deadlock/deadlock_test.go

## Bug Elements
The elements involved in the found leak are located at the following positions:

###  Conditional Variable: Wait
-> /home/ilian/Projects/Go_Fuzzing_Concurrency/Examples/Examples_Own/Deadlock/deadlock_test.go:81
```go
70 ...
71 
72 // 5) Cond deadlock: Wait with no Signal
73 func TestCondDeadlock(t *testing.T) {
74 	var mu sync.Mutex
75 	cond := sync.NewCond(&mu)
76 	go func() {
77 		time.Sleep(10 * time.Millisecond)
78 		// no cond.Signal()
79 	}()
80 	mu.Lock()
81 	cond.Wait() // blocks forever           // <-------
82 	mu.Unlock()
83 }
84 
85 // 6) Mixed deadlock: lock+channel
86 func TestMixedDeadlock(t *testing.T) {
87 	var m sync.Mutex
88 	ch := make(chan struct{})
89 
90 	go func() {
91 		m.Lock()
92 
93 ...
```


## Replay
**Replaying was not run**.

