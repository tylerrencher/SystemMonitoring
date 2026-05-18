package alerts

import "testing"

func TestApplyOp_GT(t *testing.T) {
	tests := []struct {
		val, threshold float64
		want           bool
	}{
		{101, 100, true},
		{100, 100, false}, // exactly at threshold is not strictly greater
		{99, 100, false},
		{0, 0, false},
		{3201, 3200, true},
	}
	for _, tt := range tests {
		if got := applyOp("gt", tt.val, tt.threshold, 0, 0); got != tt.want {
			t.Errorf("gt(%v, %v) = %v, want %v", tt.val, tt.threshold, got, tt.want)
		}
	}
}

func TestApplyOp_LT(t *testing.T) {
	tests := []struct {
		val, threshold float64
		want           bool
	}{
		{99, 100, true},
		{100, 100, false},
		{101, 100, false},
		{32, 33, true},
	}
	for _, tt := range tests {
		if got := applyOp("lt", tt.val, tt.threshold, 0, 0); got != tt.want {
			t.Errorf("lt(%v, %v) = %v, want %v", tt.val, tt.threshold, got, tt.want)
		}
	}
}

func TestApplyOp_Between(t *testing.T) {
	tests := []struct {
		val, low, high float64
		want           bool
	}{
		{500, 100, 2700, true},
		{100, 100, 2700, false}, // low bound is exclusive
		{2700, 100, 2700, false}, // high bound is exclusive
		{99, 100, 2700, false},
		{2701, 100, 2700, false},
		{101, 100, 2700, true},
		{2699, 100, 2700, true},
	}
	for _, tt := range tests {
		if got := applyOp("between", tt.val, 0, tt.low, tt.high); got != tt.want {
			t.Errorf("between(%v, low=%v, high=%v) = %v, want %v", tt.val, tt.low, tt.high, got, tt.want)
		}
	}
}

func TestApplyOp_UnknownOp(t *testing.T) {
	if applyOp("gte", 100, 100, 0, 0) {
		t.Error("unknown op 'gte' should return false")
	}
	if applyOp("", 100, 100, 0, 0) {
		t.Error("empty op should return false")
	}
	if applyOp("neq", 0, 1, 0, 0) {
		t.Error("unknown op 'neq' should return false")
	}
}
