package domain

// ResolveStepSize determines and validates the discrete step size relative to treasury capital.
// If requestedStepSize is nil or <= 0, it dynamically defaults to capital / 20.
// It enforces the search space guard that capital / stepSize <= 200.
func ResolveStepSize(capital float64, requestedStepSize *float64) (float64, error) {
	var stepSize float64
	if requestedStepSize == nil {
		stepSize = capital / 20.0
	} else {
		if *requestedStepSize <= 0 {
			return 0, &ValidationError{
				Details: []string{"step_size must be greater than 0"},
			}
		}
		stepSize = *requestedStepSize
	}

	if (capital / stepSize) > 200.0 {
		return 0, &ValidationError{
			Details: []string{"step_size too small for capital, max 200 steps"},
		}
	}

	return stepSize, nil
}
