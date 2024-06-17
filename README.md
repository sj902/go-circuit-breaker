# go-circuit-breaker
minimal circuit breaker written in GO

```
Timeout -> Time after which the circuit goes from open to half open
MaxRequests -> Max requests that can happen in half open state
ReadyToTrip -> Checks if cuit should be tripped
```

## Example
```
var cb *gobreaker.CircuitBreaker[[]byte]
func Get(url string) ([]byte, error) {
	body, err := cb.Execute(func() (interface{}, error) {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return body, nil
	})
	if err != nil {
		return nil, err
	}

	return body.([]byte), nil
}
```
