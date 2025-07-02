# Deadlock 

These test cases demonstrating various types of deadlocks in Go, including classic lock inversions, channel-based deadlocks, and other synchronization issues.


---

## Two-Lock Inversion (Classic Two-Goroutine Cycle)

This test demonstrates a classic two-lock inversion deadlock, where two goroutines try to lock two resources (locks) in opposite orders, causing a deadlock.

One goroutine locks `a` and then attempts to lock `b`, while the main goroutine locks `b` and then attempts to lock `a`. This inversion causes a deadlock as both goroutines are waiting on each other to release the locks.

```go
func TestCycleTwoLocks(t *testing.T) {
    var a, b sync.Mutex

    go func() {
        a.Lock()
        time.Sleep(10 * time.Millisecond)
        b.Lock()
        b.Unlock()
        a.Unlock()
    }()

    b.Lock()
    time.Sleep(5 * time.Millisecond)
    a.Lock()
    a.Unlock()
    b.Unlock()
}
```


---


## Three-Lock Cycle (A → B → C → A)
This test demonstrates a more complex deadlock with three locks forming a cycle. Goroutines acquire locks in a circular order, resulting in a deadlock.

The goroutines lock `x`, `y`, and `z` in a cycle. The main goroutine attempts to lock `z` and `x`, causing a deadlock because no goroutine can complete its lock acquisition.

```go
func TestCycleThreeLocks(t *testing.T) {
    var x, y, z sync.Mutex

    go func() {
        x.Lock()
        time.Sleep(5 * time.Millisecond)
        y.Lock()
        y.Unlock()
        x.Unlock()
    }()
    go func() {
        y.Lock()
        time.Sleep(5 * time.Millisecond)
        z.Lock()
        z.Unlock()
        y.Unlock()
    }()
    // main goroutine forms the third link:
    z.Lock()
    time.Sleep(5 * time.Millisecond)
    x.Lock() // deadlock on x
    x.Unlock()
    z.Unlock()
}
```


---


## Channel-Based Deadlock (Send + Receive Missing)

This test shows a channel-based deadlock where a goroutine sends a value to a channel, but there is no corresponding receiver.

The send operation on the channel blocks because there is no receiver to read from the channel, causing the program to deadlock.

```go
func TestChannelDeadlock(t *testing.T) {
    ch := make(chan int)
    // no corresponding receive
    ch <- 1
}

```


---


## WaitGroup Deadlock (Missing Done)
This test demonstrates a deadlock involving a `sync.WaitGroup` where the `Done()` method is not called, causing the `Wait()` to block indefinitely.

The `WaitGroup`'s `Done()` method is never called in the goroutine, so the `Wait()` in the main function blocks indefinitely, causing a deadlock.

```go
func TestWaitGroupDeadlock(t *testing.T) {
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        // forgot wg.Done()
        time.Sleep(20 * time.Millisecond)
    }()
    wg.Wait() // blocks forever
}
```


---


## Cond Deadlock (Wait with No Signal)
This test demonstrates a `sync.Cond` deadlock where the `Wait()` method is called without a corresponding `Signal()` or `Broadcast()`, causing the goroutine to block forever.

The main goroutine locks the mutex and waits on the condition variable, but since no signal is sent, the goroutine blocks indefinitely.

```go
func TestCondDeadlock(t *testing.T) {
    var mu sync.Mutex
    cond := sync.NewCond(&mu)
    go func() {
        time.Sleep(10 * time.Millisecond)
        // no cond.Signal()
    }()
    mu.Lock()
    cond.Wait() // blocks forever
    mu.Unlock()
}
```


---


## Mixed Deadlock (Lock + Channel)
This test demonstrates a mixed deadlock scenario involving both a lock and a channel. The main goroutine holds the channel send while also wanting to lock the mutex, while the goroutine holding the lock is waiting on the channel.

The main goroutine sends to the channel while also trying to lock the mutex. The goroutine already holding the mutex is waiting for the channel, causing a deadlock.

```go
func TestMixedDeadlock(t *testing.T) {
    var m sync.Mutex
    ch := make(chan struct{})

    go func() {
        m.Lock()
        defer m.Unlock()
        <-ch // waiting on channel
    }()

    // main holds the channel send, but also wants the mutex:
    ch <- struct{}{}
    m.Lock() // deadlock: goroutine holds m waiting on ch, main holds ch waiting on m
    m.Unlock()
}
```

**Comparison**:
- [Bug Types](./results/comparison_pivot_Bug_Types.csv)

- [Total Time](./results/comparison_pivot_Total_Time_s.csv)
