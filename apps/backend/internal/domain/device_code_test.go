package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeviceCode_IsExpiredOncePastItsExpiry(t *testing.T) {
	assert.False(t, (&DeviceCode{ExpiresAt: time.Now().Add(time.Minute)}).IsExpired(), "a code that expires in a minute")
	assert.True(t, (&DeviceCode{ExpiresAt: time.Now().Add(-time.Minute)}).IsExpired(), "a code that expired a minute ago")
}
