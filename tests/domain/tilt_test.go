package domain_test

import (
	"testing"

	"github.com/onesine/nevup-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSessionTiltIndex(t *testing.T) {
	require.Equal(t, 0.5, domain.SessionTiltIndex(2, 4))
	require.Equal(t, 0.0, domain.SessionTiltIndex(0, 5))
	require.Equal(t, 0.0, domain.SessionTiltIndex(0, 0))
	require.Equal(t, 1.0, domain.SessionTiltIndex(3, 3))
}
