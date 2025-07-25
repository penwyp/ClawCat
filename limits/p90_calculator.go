package limits

import (
	"fmt"
	"math"
	"sort"
)

// P90Calculator P90 百分位计算器
type P90Calculator struct {
	windowSize int
	minSamples int
}

// NewP90Calculator 创建 P90 计算器
func NewP90Calculator() *P90Calculator {
	return &P90Calculator{
		windowSize: 30, // 30天窗口
		minSamples: 10, // 最少10个样本
	}
}

// Calculate 计算 P90 值
func (p *P90Calculator) Calculate(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 复制并排序
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// 计算 90 百分位的位置
	index := int(math.Ceil(0.9 * float64(len(sorted))))
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// CalculateWithOutlierRemoval 计算 P90 并移除异常值
func (p *P90Calculator) CalculateWithOutlierRemoval(values []float64) float64 {
	if len(values) < p.minSamples {
		return p.Calculate(values)
	}

	// 计算 IQR（四分位距）
	q1 := p.percentile(values, 25)
	q3 := p.percentile(values, 75)
	iqr := q3 - q1

	// 定义异常值边界
	lowerBound := q1 - 1.5*iqr
	upperBound := q3 + 1.5*iqr

	// 过滤异常值
	filtered := []float64{}
	for _, v := range values {
		if v >= lowerBound && v <= upperBound {
			filtered = append(filtered, v)
		}
	}

	return p.Calculate(filtered)
}

// percentile 计算百分位数
func (p *P90Calculator) percentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	index := (percentile / 100) * float64(len(sorted)-1)
	lower := math.Floor(index)
	upper := math.Ceil(index)

	if lower == upper {
		return sorted[int(index)]
	}

	// 线性插值
	return sorted[int(lower)]*(upper-index) + sorted[int(upper)]*(index-lower)
}

// AnalyzeDistribution 分析数据分布
func (p *P90Calculator) AnalyzeDistribution(values []float64) Distribution {
	if len(values) == 0 {
		return Distribution{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	return Distribution{
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Mean:   p.mean(values),
		Median: p.percentile(values, 50),
		P25:    p.percentile(values, 25),
		P75:    p.percentile(values, 75),
		P90:    p.percentile(values, 90),
		P95:    p.percentile(values, 95),
		P99:    p.percentile(values, 99),
		StdDev: p.stdDev(values),
	}
}

// mean 计算平均值
func (p *P90Calculator) mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

// stdDev 计算标准差
func (p *P90Calculator) stdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	mean := p.mean(values)
	sumSquares := 0.0

	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}

	variance := sumSquares / float64(len(values)-1)
	return math.Sqrt(variance)
}

// CalculateP90Limit 计算 P90 限额
func (lm *LimitManager) CalculateP90Limit() (float64, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if len(lm.history) < lm.p90Calculator.minSamples {
		return 0, fmt.Errorf("insufficient historical data: need at least %d data points, have %d", 
			lm.p90Calculator.minSamples, len(lm.history))
	}

	// 收集历史成本数据
	costs := make([]float64, len(lm.history))
	for i, h := range lm.history {
		costs[i] = h.Cost
	}

	// 计算 P90
	p90 := lm.p90Calculator.Calculate(costs)

	// 添加 10% 的缓冲
	return p90 * 1.1, nil
}

// GetDistributionAnalysis 获取使用分布分析
func (lm *LimitManager) GetDistributionAnalysis() Distribution {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if len(lm.history) == 0 {
		return Distribution{}
	}

	costs := make([]float64, len(lm.history))
	for i, h := range lm.history {
		costs[i] = h.Cost
	}

	return lm.p90Calculator.AnalyzeDistribution(costs)
}

// GetRecommendedLimit 获取推荐限额
func (lm *LimitManager) GetRecommendedLimit() (float64, string, error) {
	if len(lm.history) < lm.p90Calculator.minSamples {
		return 0, "", fmt.Errorf("need at least %d days of historical data", lm.p90Calculator.minSamples)
	}

	costs := make([]float64, len(lm.history))
	for i, h := range lm.history {
		costs[i] = h.Cost
	}

	distribution := lm.p90Calculator.AnalyzeDistribution(costs)
	
	// 基于不同百分位提供不同的推荐
	recommendations := []struct {
		value  float64
		desc   string
		buffer float64
	}{
		{distribution.P90, "Conservative (P90)", 1.1},
		{distribution.P95, "Balanced (P95)", 1.05},
		{distribution.P99, "Liberal (P99)", 1.02},
	}

	// 选择最接近当前计划的推荐
	currentLimit := lm.plan.CostLimit
	bestRec := recommendations[0] // 默认保守推荐
	
	if currentLimit > 0 {
		minDiff := math.Abs(recommendations[0].value*recommendations[0].buffer - currentLimit)
		for _, rec := range recommendations {
			diff := math.Abs(rec.value*rec.buffer - currentLimit)
			if diff < minDiff {
				minDiff = diff
				bestRec = rec
			}
		}
	}

	recommendedLimit := bestRec.value * bestRec.buffer
	return recommendedLimit, bestRec.desc, nil
}

// ValidateHistoricalData 验证历史数据质量
func (p *P90Calculator) ValidateHistoricalData(values []float64) (bool, []string) {
	issues := []string{}
	
	if len(values) < p.minSamples {
		issues = append(issues, fmt.Sprintf("Insufficient data points: %d (minimum %d required)", len(values), p.minSamples))
		return false, issues
	}

	// 检查数据变异性
	if len(values) > 0 {
		distribution := p.AnalyzeDistribution(values)
		
		// 检查是否有过多的零值
		zeros := 0
		for _, v := range values {
			if v == 0 {
				zeros++
			}
		}
		
		if float64(zeros)/float64(len(values)) > 0.5 {
			issues = append(issues, "More than 50% of data points are zero")
		}

		// 检查标准差是否过小（数据变化太少）
		if distribution.StdDev < distribution.Mean*0.1 {
			issues = append(issues, "Low data variability detected")
		}

		// 检查是否有极端异常值
		q1 := distribution.P25
		q3 := distribution.P75
		iqr := q3 - q1
		outlierCount := 0
		
		for _, v := range values {
			if v < q1-3*iqr || v > q3+3*iqr {
				outlierCount++
			}
		}
		
		if float64(outlierCount)/float64(len(values)) > 0.1 {
			issues = append(issues, "High number of extreme outliers detected")
		}
	}

	return len(issues) == 0, issues
}