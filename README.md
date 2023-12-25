# go-circuit-breaker
minimal circuit breaker written in GO

```
Timeout -> Time after which the circuit goes from open to half open
MaxRequests -> Max requests that can happen in half open state
ReadyToTrip -> Checks if cuit should be tripped
```
