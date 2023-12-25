package breaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type State int

const (
	StateHalfOpen State = iota
	StateOpen
	StateClosed
)

var (
	// ErrTooManyRequests is returned when the CB state is half open and the requests count is over the cb maxRequests
	ErrTooManyRequests = errors.New("too many requests")
	// ErrOpenState is returned when the CB state is open
	ErrOpenState = errors.New("circuit breaker is open")
)

// String implements stringer interface.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return fmt.Sprintf("unknown state: %d", s)
	}
}

type Counts struct {
	Requests           int
	TotalSuccess       int
	TotalFail          int
	ConsecutiveSuccess int
	ConsecutiveFail    int
}

func (c *Counts) onRequest() {
	c.Requests++
}

func (c *Counts) onSuccess() {
	c.ConsecutiveSuccess++
	c.TotalSuccess++
	c.ConsecutiveFail = 0
}

func (c *Counts) onFail() {
	c.ConsecutiveFail++
	c.TotalFail++
	c.ConsecutiveSuccess = 0
}

func (c *Counts) clear() {
	c.Requests = 0
	c.TotalSuccess = 0
	c.TotalFail = 0
	c.ConsecutiveSuccess = 0
	c.ConsecutiveFail = 0
}

type Settings struct {
	Timeout     time.Duration
	MaxRequests int
	ReadyToTrip func(c Counts) bool
}

type CircuitBreaker struct {
	timeout     time.Duration
	maxRequests int
	readyToTrip func(c Counts) bool

	mutex      sync.Mutex
	state      State
	generation int
	counts     Counts
	expiry     time.Time
}

const defaultTimeOut = 60 * time.Second
const defaultMaxRequests = 5

func defaultReadyToTrip(c Counts) bool {
	return c.ConsecutiveFail >= 5
}

func NewCircuitBreaker(setings Settings) *CircuitBreaker {
	cb := new(CircuitBreaker)

	if setings.Timeout <= 0 {
		cb.timeout = defaultTimeOut
	} else {
		cb.timeout = setings.Timeout
	}

	if setings.Timeout <= 0 {
		cb.maxRequests = defaultMaxRequests
	} else {
		cb.maxRequests = setings.MaxRequests
	}

	if setings.ReadyToTrip == nil {
		cb.readyToTrip = defaultReadyToTrip
	} else {
		cb.readyToTrip = setings.ReadyToTrip
	}

	cb.refresh(time.Now())

	cb.state = StateClosed

	cb.generation = 0

	return cb
}

func (cb *CircuitBreaker) refresh(t time.Time) {
	cb.generation++
	cb.counts.clear()
	var zero = time.Time{}
	switch cb.state {
	case StateClosed:
		cb.expiry = t.Add(cb.timeout)
	default:
		cb.expiry = zero
	}
}

func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()

	if err != nil {
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	res, err := req()
	cb.afterRequest(generation, err != nil)

	return res, err
}

func (cb *CircuitBreaker) beforeRequest() (int, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.counts.onRequest()
	currState, generation := cb.currentState(time.Now())
	if currState == StateOpen {
		return generation, ErrOpenState
	}
	if currState == StateHalfOpen && cb.counts.Requests > cb.maxRequests {
		return generation, ErrTooManyRequests
	}

	return generation, nil
}

func (cb *CircuitBreaker) afterRequest(before int, isSuccess bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	currState, generation := cb.currentState(time.Now())

	if generation != before {
		return
	}

	if isSuccess {
		cb.onSuccess(currState, now)
	} else {
		cb.onFail(currState, now)
	}
}

func (cb *CircuitBreaker) onSuccess(currState State, t time.Time) {
	switch currState {
	case StateClosed:
		cb.counts.onSuccess()
		if cb.readyToTrip(cb.counts) {
			cb.setState(StateOpen, t)
		}
	case StateHalfOpen:
		cb.counts.onSuccess()
		if cb.counts.ConsecutiveSuccess >= cb.maxRequests {
			cb.setState(StateClosed, t)
		}
	}
}

func (cb *CircuitBreaker) onFail(currState State, t time.Time) {
	switch currState {
	case StateClosed:
		cb.counts.onFail()
	case StateHalfOpen:
		cb.counts.onFail()
		cb.setState(StateOpen, t)

	}
}

func (cb *CircuitBreaker) currentState(t time.Time) (State, int) {
	if cb.state == StateClosed && cb.expiry.Before(t) {
		cb.setState(StateHalfOpen, time.Now())
	}
	return cb.state, int(cb.generation)
}

func (cb *CircuitBreaker) setState(s State, t time.Time) {
	if s == cb.state {
		return
	}

	cb.state = s
	cb.newGeneration(t)
}

func (cb *CircuitBreaker) newGeneration(t time.Time) {
	cb.counts.clear()
	cb.generation++

	var zero time.Time

	if cb.state == StateOpen {
		cb.expiry = t.Add(cb.timeout)
	} else {
		cb.expiry = zero
	}
}
