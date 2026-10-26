package socks5lb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetEnv(t *testing.T) {
	for _, tt := range []struct {
		value, want string
	}{
		{"", "default"},
		{" \t ", "default"},
		{" value \n", "value"},
	} {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("SOCKS5LB_TEST_VALUE", tt.value)
			require.Equal(t, tt.want, GetEnv("SOCKS5LB_TEST_VALUE", "default"))
		})
	}
}

func TestSecFromEnv(t *testing.T) {
	for _, tt := range []struct {
		name, value string
		want        time.Duration
	}{
		{"unset", "", 5 * time.Second},
		{"whitespace", " \t", 5 * time.Second},
		{"valid", " 2 ", 2 * time.Second},
		{"zero", "0", 5 * time.Second},
		{"negative", "-1", 5 * time.Second},
		{"invalid", "invalid", 5 * time.Second},
		{"duration overflow", "9223372037", 5 * time.Second},
		{"integer overflow", "18446744073709551616", 5 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SOCKS5LB_TEST_INTERVAL", tt.value)
			require.Equal(t, tt.want, SecFromEnv("SOCKS5LB_TEST_INTERVAL", 5))
		})
	}
}

func TestSecFromEnvInvalidDefault(t *testing.T) {
	t.Setenv("SOCKS5LB_TEST_INTERVAL", "")
	for _, value := range []uint64{0, ^uint64(0)} {
		require.Equal(t, time.Second, SecFromEnv("SOCKS5LB_TEST_INTERVAL", value))
	}
}

func TestDebugModeFromEnv(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"", false},
		{"true", true},
		{" true ", true},
		{"false", false},
		{"invalid", false},
	} {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("DEBUG", tt.value)
			require.Equal(t, tt.want, debugModeFromEnv())
		})
	}
}
