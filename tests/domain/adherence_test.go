package domain_test

import (
	"testing"

	"github.com/onesine/nevup-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRollingAvg(t *testing.T) {
	require.Equal(t, 3.0, domain.RollingAvg([]int{3, 3, 3, 3, 3}))
	require.Equal(t, 2.5, domain.RollingAvg([]int{1, 2, 3, 4}))
	require.Equal(t, 0.0, domain.RollingAvg([]int{}))
	// Only last 10 considered even if more passed:
	ratings := []int{1, 1, 1, 1, 1, 5, 5, 5, 5, 5}
	require.Equal(t, 3.0, domain.RollingAvg(ratings))
}
