package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/sj902/breaker"
)

var cb *breaker.CircuitBreaker

func init() {
	var st breaker.Settings
	st.ReadyToTrip = func(counts breaker.Counts) bool {
		failureRatio := float64(counts.TotalFail) / float64(counts.Requests)
		return counts.Requests >= 3 && failureRatio >= 0.5
	}

	cb = breaker.NewCircuitBreaker(st)
}

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

func main() {
	body, err := Get("http://www.google.com/robots.txt")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(body))
}
