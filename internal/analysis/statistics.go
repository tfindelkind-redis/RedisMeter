// Package analysis provides performance analysis and comparison capabilities.
package analysis

import (
	"math"
	"sort"
)

// Statistics contains statistical measures for a set of values.
type Statistics struct {
	Count      int     `json:"count"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Mean       float64 `json:"mean"`
	Median     float64 `json:"median"`
	StdDev     float64 `json:"std_dev"`
	Variance   float64 `json:"variance"`
	P50        float64 `json:"p50"`
	P90        float64 `json:"p90"`
	P95        float64 `json:"p95"`
	P99        float64 `json:"p99"`
	CoeffVar   float64 `json:"coeff_var"` // coefficient of variation
}

// CalculateStatistics computes statistical measures for a slice of values.
func CalculateStatistics(values []float64) *Statistics {
	if len(values) == 0 {
		return &Statistics{}
	}

	stats := &Statistics{
		Count: len(values),
	}

	// Sort for percentile calculations
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// Min/Max
	stats.Min = sorted[0]
	stats.Max = sorted[len(sorted)-1]

	// Mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	stats.Mean = sum / float64(len(values))

	// Variance and StdDev
	sumSquares := 0.0
	for _, v := range values {
		diff := v - stats.Mean
		sumSquares += diff * diff
	}
	stats.Variance = sumSquares / float64(len(values))
	stats.StdDev = math.Sqrt(stats.Variance)

	// Coefficient of variation
	if stats.Mean != 0 {
		stats.CoeffVar = (stats.StdDev / stats.Mean) * 100
	}

	// Percentiles
	stats.Median = percentile(sorted, 50)
	stats.P50 = stats.Median
	stats.P90 = percentile(sorted, 90)
	stats.P95 = percentile(sorted, 95)
	stats.P99 = percentile(sorted, 99)

	return stats
}

// percentile calculates the p-th percentile of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	// Use linear interpolation
	rank := (p / 100) * float64(len(sorted)-1)
	lower := int(rank)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	fraction := rank - float64(lower)
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}

// TTest performs a two-sample t-test and returns the t-statistic and p-value.
// It tests whether the means of two groups are significantly different.
type TTestResult struct {
	TStatistic    float64 `json:"t_statistic"`
	PValue        float64 `json:"p_value"`
	DegreesOfFreedom int  `json:"degrees_of_freedom"`
	Significant   bool    `json:"significant"` // at alpha=0.05
	MeanDiff      float64 `json:"mean_diff"`
}

// TwoSampleTTest performs an independent two-sample t-test.
func TwoSampleTTest(sample1, sample2 []float64) *TTestResult {
	if len(sample1) < 2 || len(sample2) < 2 {
		return &TTestResult{}
	}

	n1 := float64(len(sample1))
	n2 := float64(len(sample2))

	// Calculate means
	mean1 := mean(sample1)
	mean2 := mean(sample2)

	// Calculate variances
	var1 := variance(sample1, mean1)
	var2 := variance(sample2, mean2)

	// Pooled standard error
	se := math.Sqrt((var1/n1) + (var2/n2))
	if se == 0 {
		return &TTestResult{MeanDiff: mean1 - mean2}
	}

	// T-statistic
	t := (mean1 - mean2) / se

	// Degrees of freedom (Welch's approximation)
	num := math.Pow((var1/n1)+(var2/n2), 2)
	denom := (math.Pow(var1/n1, 2)/(n1-1)) + (math.Pow(var2/n2, 2)/(n2-1))
	df := int(num / denom)
	if df < 1 {
		df = 1
	}

	// P-value approximation (using t-distribution approximation)
	pValue := tDistributionPValue(math.Abs(t), df)

	return &TTestResult{
		TStatistic:       t,
		PValue:           pValue,
		DegreesOfFreedom: df,
		Significant:      pValue < 0.05,
		MeanDiff:         mean1 - mean2,
	}
}

// mean calculates the arithmetic mean.
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// variance calculates the sample variance.
func variance(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sumSquares := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	return sumSquares / float64(len(values)-1)
}

// tDistributionPValue approximates the two-tailed p-value for a t-distribution.
// This is a simple approximation - for production use, consider a statistics library.
func tDistributionPValue(t float64, df int) float64 {
	// Use a simple approximation based on the normal distribution for large df
	if df > 30 {
		// For large df, t-distribution approaches normal distribution
		return 2 * normalCDF(-t)
	}

	// For smaller df, use a rough approximation
	// This is not highly accurate but sufficient for basic significance testing
	x := float64(df) / (float64(df) + t*t)
	return betaIncomplete(float64(df)/2.0, 0.5, x)
}

// normalCDF calculates the cumulative distribution function of standard normal.
func normalCDF(x float64) float64 {
	return 0.5 * (1 + erf(x/math.Sqrt2))
}

// erf is the error function (approximation).
func erf(x float64) float64 {
	// Approximation using Horner's method
	a1 := 0.254829592
	a2 := -0.284496736
	a3 := 1.421413741
	a4 := -1.453152027
	a5 := 1.061405429
	p := 0.3275911

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = math.Abs(x)

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return sign * y
}

// betaIncomplete calculates the incomplete beta function (approximation).
func betaIncomplete(a, b, x float64) float64 {
	// Simple approximation for the incomplete beta function
	// For production, use a proper implementation
	if x == 0 {
		return 0
	}
	if x == 1 {
		return 1
	}

	// Use continued fraction approximation
	// This is a simplified version
	const maxIter = 100
	const epsilon = 1e-10

	bt := 0.0
	if x > 0 && x < 1 {
		lg1, _ := math.Lgamma(a + b)
		lg2, _ := math.Lgamma(a)
		lg3, _ := math.Lgamma(b)
		bt = math.Exp(lg1 - lg2 - lg3 + a*math.Log(x) + b*math.Log(1-x))
	}

	if x < (a+1)/(a+b+2) {
		return bt * betaCF(a, b, x, maxIter, epsilon) / a
	}
	return 1 - bt*betaCF(b, a, 1-x, maxIter, epsilon)/b
}

// betaCF calculates continued fraction for incomplete beta function.
func betaCF(a, b, x float64, maxIter int, epsilon float64) float64 {
	qab := a + b
	qap := a + 1
	qam := a - 1
	c := 1.0
	d := 1 - qab*x/qap
	if math.Abs(d) < 1e-30 {
		d = 1e-30
	}
	d = 1 / d
	h := d

	for m := 1; m <= maxIter; m++ {
		m2 := 2 * m
		aa := float64(m) * (b - float64(m)) * x / ((qam + float64(m2)) * (a + float64(m2)))
		d = 1 + aa*d
		if math.Abs(d) < 1e-30 {
			d = 1e-30
		}
		c = 1 + aa/c
		if math.Abs(c) < 1e-30 {
			c = 1e-30
		}
		d = 1 / d
		h *= d * c

		aa = -(a + float64(m)) * (qab + float64(m)) * x / ((a + float64(m2)) * (qap + float64(m2)))
		d = 1 + aa*d
		if math.Abs(d) < 1e-30 {
			d = 1e-30
		}
		c = 1 + aa/c
		if math.Abs(c) < 1e-30 {
			c = 1e-30
		}
		d = 1 / d
		del := d * c
		h *= del

		if math.Abs(del-1) < epsilon {
			break
		}
	}

	return h
}

// ConfidenceInterval calculates a confidence interval for the mean.
type ConfidenceInterval struct {
	Lower      float64 `json:"lower"`
	Upper      float64 `json:"upper"`
	Mean       float64 `json:"mean"`
	Confidence float64 `json:"confidence"` // e.g., 0.95 for 95%
}

// CalculateConfidenceInterval calculates the confidence interval for a sample mean.
func CalculateConfidenceInterval(values []float64, confidence float64) *ConfidenceInterval {
	if len(values) < 2 {
		m := 0.0
		if len(values) == 1 {
			m = values[0]
		}
		return &ConfidenceInterval{Lower: m, Upper: m, Mean: m, Confidence: confidence}
	}

	n := float64(len(values))
	m := mean(values)
	s := math.Sqrt(variance(values, m))
	se := s / math.Sqrt(n)

	// Get t-value for the given confidence level
	// For simplicity, use approximate t-values
	alpha := 1 - confidence
	tValue := tValue(int(n)-1, alpha/2)

	margin := tValue * se

	return &ConfidenceInterval{
		Lower:      m - margin,
		Upper:      m + margin,
		Mean:       m,
		Confidence: confidence,
	}
}

// tValue returns approximate t-value for given df and alpha.
func tValue(df int, alpha float64) float64 {
	// Common t-values for 95% confidence (alpha=0.025)
	if alpha <= 0.025 {
		if df >= 120 {
			return 1.980
		} else if df >= 60 {
			return 2.000
		} else if df >= 30 {
			return 2.042
		} else if df >= 20 {
			return 2.086
		} else if df >= 10 {
			return 2.228
		} else if df >= 5 {
			return 2.571
		}
		return 2.776 // df=4
	}
	// For alpha=0.05 (90% confidence)
	if df >= 30 {
		return 1.697
	}
	return 1.833
}
