package backoffpolicy // import "github.com/condrove10/retryablehttp/backoffpolicy"

import (
	"fmt"
	"math"
	"time"
)

type Strategy string

const (
	StrategyLinear      Strategy = "Linear"
	StrategyExponential Strategy = "Exponential"
)

func BackoffPolicy(strategy Strategy, attempts uint32, delay time.Duration, policy func(attempt uint32) error) error {
	var (
		err     error
		attempt uint32
		base    uint32
	)

	switch strategy {
	case StrategyExponential:
		base = 2
	case StrategyLinear:
		base = 1
	default:
		return fmt.Errorf("invalid backoff strategy")
	}

	for ; attempt < attempts; attempt++ {
		err = policy(attempt)
		if err == nil {
			return nil
		}

		time.Sleep(delay * time.Duration(math.Pow(float64(base), float64(attempt))))
	}

	return fmt.Errorf("backoff policy exhausted: %w", err)
}
