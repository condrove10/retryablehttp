package backoffpolicy // import "github.com/condrove10/retryablehttp/backoffpolicy"

import (
	"errors"
	"fmt"
	"math"
	"time"
)

type Strategy string

const (
	StrategyLinear      Strategy = "Linear"
	StrategyExponential Strategy = "Exponential"
)

func ValidateStrategy(s Strategy) error {
	allowedStrategies := []Strategy{StrategyLinear, StrategyExponential}

	for _, allowed := range allowedStrategies {
		if s == allowed {
			return nil
		}
	}
	return errors.New("invalid strategy: " + string(s))
}

func BackoffPolicy(strategy Strategy, attempts uint32, delay time.Duration, policy func(attempt uint32) error) error {
	if err := ValidateStrategy(strategy); err != nil {
		return fmt.Errorf("strategy validation failed: %w", err)
	}

	var (
		err     error
		attempt uint32
	)

	for ; attempt < attempts; attempt++ {
		err = policy(attempt)
		if err == nil {
			return nil
		}

		switch strategy {
		case StrategyExponential:
			time.Sleep(delay * time.Duration(math.Pow(2, float64(attempt))))
		case StrategyLinear:
			time.Sleep(delay)
		default:
			return fmt.Errorf("invalid backoff strategy")
		}
	}

	return fmt.Errorf("backoff policy exhausted: %w", err)
}
