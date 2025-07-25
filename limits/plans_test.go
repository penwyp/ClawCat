package limits

import (
	"fmt"
	"testing"
)

func TestGetPlanByType(t *testing.T) {
	tests := []struct {
		planType    PlanType
		expectFound bool
		expectName  string
		expectLimit float64
	}{
		{PlanFree, true, "Free", 0.00},
		{PlanPro, true, "Pro", 18.00},
		{PlanMax5, true, "Max-5", 35.00},
		{PlanMax20, true, "Max-20", 140.00},
		{PlanCustom, true, "Custom", 0.00},
		{PlanType("invalid"), false, "", 0.00},
	}

	for _, tt := range tests {
		t.Run(string(tt.planType), func(t *testing.T) {
			plan, found := GetPlanByType(tt.planType)
			
			if found != tt.expectFound {
				t.Errorf("GetPlanByType(%v) found = %v, want %v", tt.planType, found, tt.expectFound)
			}
			
			if found {
				if plan.Name != tt.expectName {
					t.Errorf("Plan name = %v, want %v", plan.Name, tt.expectName)
				}
				if plan.CostLimit != tt.expectLimit {
					t.Errorf("Plan cost limit = %v, want %v", plan.CostLimit, tt.expectLimit)
				}
				if plan.Type != tt.planType {
					t.Errorf("Plan type = %v, want %v", plan.Type, tt.planType)
				}
			}
		})
	}
}

func TestGetAvailablePlans(t *testing.T) {
	plans := GetAvailablePlans()
	
	expectedCount := 5 // Free, Pro, Max5, Max20, Custom
	if len(plans) != expectedCount {
		t.Errorf("Expected %d plans, got %d", expectedCount, len(plans))
	}
	
	// Check that all plan types are represented
	typeMap := make(map[PlanType]bool)
	for _, plan := range plans {
		typeMap[plan.Type] = true
	}
	
	expectedTypes := []PlanType{PlanFree, PlanPro, PlanMax5, PlanMax20, PlanCustom}
	for _, expectedType := range expectedTypes {
		if !typeMap[expectedType] {
			t.Errorf("Plan type %v not found in available plans", expectedType)
		}
	}
}

func TestCreateCustomPlan(t *testing.T) {
	testCases := []struct {
		costLimit    float64
		expectedName string
	}{
		{50.0, "Custom"},
		{100.0, "Custom"},
		{25.5, "Custom"},
	}
	
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("limit_%.1f", tc.costLimit), func(t *testing.T) {
			plan := CreateCustomPlan(tc.costLimit)
			
			if plan.Name != tc.expectedName {
				t.Errorf("Expected name %v, got %v", tc.expectedName, plan.Name)
			}
			
			if plan.Type != PlanCustom {
				t.Errorf("Expected type %v, got %v", PlanCustom, plan.Type)
			}
			
			if plan.CostLimit != tc.costLimit {
				t.Errorf("Expected cost limit %v, got %v", tc.costLimit, plan.CostLimit)
			}
			
			if !plan.CustomLimit {
				t.Error("Expected CustomLimit to be true")
			}
			
			// Check token limit estimation
			avgTokenCost := 0.000015
			expectedTokenLimit := int64(tc.costLimit / avgTokenCost)
			if plan.TokenLimit != expectedTokenLimit {
				t.Errorf("Expected token limit %v, got %v", expectedTokenLimit, plan.TokenLimit)
			}
			
			// Check that features are set
			if len(plan.Features) == 0 {
				t.Error("Expected custom plan to have features")
			}
			
			// Check that warning levels are set
			if len(plan.WarningLevels) == 0 {
				t.Error("Expected custom plan to have warning levels")
			}
		})
	}
}

func TestIsValidPlanType(t *testing.T) {
	validTypes := []string{"free", "pro", "max5", "max20", "custom"}
	invalidTypes := []string{"invalid", "premium", "enterprise", ""}
	
	for _, validType := range validTypes {
		t.Run("valid_"+validType, func(t *testing.T) {
			if !IsValidPlanType(validType) {
				t.Errorf("Expected %v to be valid", validType)
			}
		})
	}
	
	for _, invalidType := range invalidTypes {
		t.Run("invalid_"+invalidType, func(t *testing.T) {
			if IsValidPlanType(invalidType) {
				t.Errorf("Expected %v to be invalid", invalidType)
			}
		})
	}
}

func TestGetPlanFeatures(t *testing.T) {
	tests := []struct {
		planType       PlanType
		expectedFeatures int
	}{
		{PlanFree, 1},    // Basic usage tracking
		{PlanPro, 3},     // 5-hour sessions, All models, Priority support
		{PlanMax5, 4},    // Pro features + Advanced analytics
		{PlanMax20, 5},   // Max5 features + Team features
		{PlanCustom, 3},  // P90-based limits, Custom thresholds, Advanced monitoring
	}
	
	for _, tt := range tests {
		t.Run(string(tt.planType), func(t *testing.T) {
			features := GetPlanFeatures(tt.planType)
			
			if len(features) != tt.expectedFeatures {
				t.Errorf("Expected %d features for %v, got %d: %v", 
					tt.expectedFeatures, tt.planType, len(features), features)
			}
		})
	}
	
	// Test invalid plan type
	features := GetPlanFeatures(PlanType("invalid"))
	if len(features) != 0 {
		t.Errorf("Expected 0 features for invalid plan, got %d", len(features))
	}
}

func TestComparePlans(t *testing.T) {
	tests := []struct {
		plan1    PlanType
		plan2    PlanType
		expected int
	}{
		{PlanFree, PlanPro, -1},      // Free < Pro
		{PlanPro, PlanFree, 1},       // Pro > Free
		{PlanPro, PlanPro, 0},        // Pro == Pro
		{PlanPro, PlanMax5, -1},      // Pro < Max5
		{PlanMax5, PlanMax20, -1},    // Max5 < Max20
		{PlanMax20, PlanCustom, -1},  // Max20 < Custom
		{PlanCustom, PlanFree, 1},    // Custom > Free
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v_vs_%v", tt.plan1, tt.plan2), func(t *testing.T) {
			result := ComparePlans(tt.plan1, tt.plan2)
			if result != tt.expected {
				t.Errorf("ComparePlans(%v, %v) = %d, want %d", tt.plan1, tt.plan2, result, tt.expected)
			}
		})
	}
	
	// Test invalid plan types
	result := ComparePlans(PlanType("invalid1"), PlanType("invalid2"))
	if result != 0 {
		t.Errorf("Expected 0 for invalid plan comparison, got %d", result)
	}
}

func TestWarningLevelsStructure(t *testing.T) {
	// Test that all predefined plans have proper warning levels
	plans := []PlanType{PlanFree, PlanPro, PlanMax5, PlanMax20, PlanCustom}
	
	for _, planType := range plans {
		t.Run(string(planType), func(t *testing.T) {
			plan, found := GetPlanByType(planType)
			if !found {
				t.Fatalf("Plan %v should exist", planType)
			}
			
			if len(plan.WarningLevels) == 0 {
				t.Errorf("Plan %v should have warning levels", planType)
				return
			}
			
			// Check that warning levels are in ascending order
			for i := 1; i < len(plan.WarningLevels); i++ {
				if plan.WarningLevels[i].Threshold <= plan.WarningLevels[i-1].Threshold {
					t.Errorf("Warning levels should be in ascending order for plan %v", planType)
				}
			}
			
			// Check that each warning level has required fields
			for i, level := range plan.WarningLevels {
				if level.Threshold <= 0 || level.Threshold > 100 {
					t.Errorf("Warning level %d threshold should be between 0-100, got %.1f", i, level.Threshold)
				}
				
				if level.Message == "" {
					t.Errorf("Warning level %d should have a message", i)
				}
				
				if len(level.Actions) == 0 {
					t.Errorf("Warning level %d should have actions", i)
				}
				
				// Check that severity is valid
				validSeverities := []Severity{SeverityInfo, SeverityWarning, SeverityError, SeverityCritical}
				found := false
				for _, validSev := range validSeverities {
					if level.Severity == validSev {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Warning level %d has invalid severity: %v", i, level.Severity)
				}
			}
		})
	}
}

func TestCreateDefaultWarningLevels(t *testing.T) {
	testLimits := []float64{10.0, 50.0, 100.0}
	
	for _, limit := range testLimits {
		t.Run(fmt.Sprintf("limit_%.1f", limit), func(t *testing.T) {
			levels := createDefaultWarningLevels(limit)
			
			if len(levels) != 4 {
				t.Errorf("Expected 4 warning levels, got %d", len(levels))
			}
			
			expectedThresholds := []float64{75, 90, 95, 100}
			for i, expectedThreshold := range expectedThresholds {
				if i < len(levels) && levels[i].Threshold != expectedThreshold {
					t.Errorf("Expected threshold %.1f at index %d, got %.1f", 
						expectedThreshold, i, levels[i].Threshold)
				}
			}
			
			// Check severity progression
			expectedSeverities := []Severity{SeverityInfo, SeverityWarning, SeverityError, SeverityCritical}
			for i, expectedSeverity := range expectedSeverities {
				if i < len(levels) && levels[i].Severity != expectedSeverity {
					t.Errorf("Expected severity %v at index %d, got %v", 
						expectedSeverity, i, levels[i].Severity)
				}
			}
		})
	}
}

func TestPlanResetCycles(t *testing.T) {
	plans := GetAvailablePlans()
	
	for _, plan := range plans {
		t.Run(string(plan.Type), func(t *testing.T) {
			// All current plans should have monthly reset cycle
			if plan.ResetCycle != ResetCycleMonthly {
				t.Errorf("Plan %v should have monthly reset cycle, got %v", plan.Type, plan.ResetCycle)
			}
		})
	}
}

func TestPlanTokenLimits(t *testing.T) {
	tests := []struct {
		planType     PlanType
		expectTokens bool
	}{
		{PlanFree, false},    // Free plan has 0 token limit
		{PlanPro, true},      // Pro plan has token limit
		{PlanMax5, true},     // Max5 plan has token limit  
		{PlanMax20, true},    // Max20 plan has token limit
		{PlanCustom, false},  // Custom plan starts with 0, gets calculated
	}
	
	for _, tt := range tests {
		t.Run(string(tt.planType), func(t *testing.T) {
			plan, found := GetPlanByType(tt.planType)
			if !found {
				t.Fatalf("Plan %v should exist", tt.planType)
			}
			
			hasTokens := plan.TokenLimit > 0
			if hasTokens != tt.expectTokens {
				t.Errorf("Plan %v token limit expectation mismatch: has=%v, expect=%v (limit=%d)", 
					tt.planType, hasTokens, tt.expectTokens, plan.TokenLimit)
			}
		})
	}
}