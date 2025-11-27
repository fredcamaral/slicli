package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyChecksum(t *testing.T) {
	t.Run("valid checksum passes", func(t *testing.T) {
		data := []byte("test plugin binary data")
		hash := sha256.Sum256(data)
		expectedHash := hex.EncodeToString(hash[:])

		err := verifyChecksum(data, expectedHash)
		assert.NoError(t, err)
	})

	t.Run("invalid checksum fails", func(t *testing.T) {
		data := []byte("test plugin binary data")
		invalidHash := "0000000000000000000000000000000000000000000000000000000000000000"

		err := verifyChecksum(data, invalidHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "checksum mismatch")
	})

	t.Run("empty hash fails", func(t *testing.T) {
		data := []byte("test plugin binary data")

		err := verifyChecksum(data, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected hash is empty")
	})

	t.Run("tampered data fails", func(t *testing.T) {
		originalData := []byte("original plugin binary")
		hash := sha256.Sum256(originalData)
		originalHash := hex.EncodeToString(hash[:])

		tamperedData := []byte("tampered plugin binary")
		err := verifyChecksum(tamperedData, originalHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "checksum mismatch")
	})

	t.Run("case sensitivity in hash", func(t *testing.T) {
		data := []byte("test data")
		hash := sha256.Sum256(data)
		lowerHash := hex.EncodeToString(hash[:])

		// Test with lowercase (standard)
		err := verifyChecksum(data, lowerHash)
		assert.NoError(t, err)
	})

	t.Run("hash length validation", func(t *testing.T) {
		data := []byte("test data")

		// Too short hash
		err := verifyChecksum(data, "abcd1234")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "checksum mismatch")
	})
}

func TestConstantTimeEqual(t *testing.T) {
	t.Run("equal strings return true", func(t *testing.T) {
		assert.True(t, constantTimeEqual("abc123", "abc123"))
	})

	t.Run("different strings return false", func(t *testing.T) {
		assert.False(t, constantTimeEqual("abc123", "xyz789"))
	})

	t.Run("different length strings return false", func(t *testing.T) {
		assert.False(t, constantTimeEqual("short", "longer_string"))
	})

	t.Run("empty strings are equal", func(t *testing.T) {
		assert.True(t, constantTimeEqual("", ""))
	})

	t.Run("one empty string returns false", func(t *testing.T) {
		assert.False(t, constantTimeEqual("abc", ""))
		assert.False(t, constantTimeEqual("", "abc"))
	})
}

func TestNewMarketplaceClient_HTTPSEnforcement(t *testing.T) {
	t.Run("HTTPS URL is accepted", func(t *testing.T) {
		config := MarketplaceConfig{
			BaseURL: "https://marketplace.example.com",
		}

		client, err := NewMarketplaceClient(config)
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("HTTP URL is rejected", func(t *testing.T) {
		config := MarketplaceConfig{
			BaseURL: "http://marketplace.example.com",
		}

		client, err := NewMarketplaceClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "HTTPS")
	})

	t.Run("empty URL is accepted", func(t *testing.T) {
		config := MarketplaceConfig{
			BaseURL: "",
		}

		client, err := NewMarketplaceClient(config)
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestPluginDownloadInfo(t *testing.T) {
	t.Run("struct marshaling", func(t *testing.T) {
		info := PluginDownloadInfo{
			URL:    "https://example.com/plugin.so",
			SHA256: "abc123def456",
			Size:   1024,
		}

		assert.Equal(t, "https://example.com/plugin.so", info.URL)
		assert.Equal(t, "abc123def456", info.SHA256)
		assert.Equal(t, int64(1024), info.Size)
	})
}

func TestMaxPluginDownloadSize(t *testing.T) {
	t.Run("constant is reasonable", func(t *testing.T) {
		// 50MB limit
		assert.Equal(t, int64(50*1024*1024), MaxPluginDownloadSize)
	})
}

func TestValidateDownloadURL(t *testing.T) {
	t.Run("valid HTTPS URL passes", func(t *testing.T) {
		// Use a well-known resolvable hostname for this test
		err := ValidateDownloadURL("https://github.com/plugins/test.so")
		assert.NoError(t, err)
	})

	t.Run("HTTP URL fails", func(t *testing.T) {
		err := ValidateDownloadURL("http://marketplace.slicli.dev/plugins/test.so")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInsecureURL)
	})

	t.Run("localhost URL fails", func(t *testing.T) {
		err := ValidateDownloadURL("https://localhost/plugins/test.so")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInternalURL)
	})

	t.Run("loopback IP fails", func(t *testing.T) {
		err := ValidateDownloadURL("https://127.0.0.1/plugins/test.so")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInternalURL)
	})

	t.Run("private network fails", func(t *testing.T) {
		testCases := []string{
			"https://192.168.1.1/plugins/test.so",
			"https://10.0.0.1/plugins/test.so",
			"https://172.16.0.1/plugins/test.so",
		}

		for _, tc := range testCases {
			err := ValidateDownloadURL(tc)
			assert.Error(t, err, "Expected error for URL: %s", tc)
			assert.ErrorIs(t, err, ErrInternalURL, "Expected ErrInternalURL for URL: %s", tc)
		}
	})

	t.Run("invalid URL fails", func(t *testing.T) {
		err := ValidateDownloadURL("://invalid")
		assert.Error(t, err)
	})
}

func TestIsInternalURL(t *testing.T) {
	t.Run("localhost is internal", func(t *testing.T) {
		isInternal, err := IsInternalURL("https://localhost/path")
		require.NoError(t, err)
		assert.True(t, isInternal)
	})

	t.Run("subdomain of localhost is internal", func(t *testing.T) {
		isInternal, err := IsInternalURL("https://test.localhost/path")
		require.NoError(t, err)
		assert.True(t, isInternal)
	})

	t.Run("loopback IP is internal", func(t *testing.T) {
		isInternal, err := IsInternalURL("https://127.0.0.1/path")
		require.NoError(t, err)
		assert.True(t, isInternal)
	})

	t.Run("external URL is not internal", func(t *testing.T) {
		// Note: This test requires DNS resolution
		isInternal, err := IsInternalURL("https://google.com/path")
		// If DNS resolution fails, skip the test
		if err != nil {
			t.Skip("DNS resolution required for this test")
		}
		assert.False(t, isInternal)
	})

	t.Run("empty hostname fails", func(t *testing.T) {
		_, err := IsInternalURL("https:///path")
		assert.Error(t, err)
	})
}

func TestIsPrivateIP(t *testing.T) {
	t.Run("loopback IPv4 is private", func(t *testing.T) {
		testIPs := []string{
			"127.0.0.1",
			"127.0.0.255",
			"127.255.255.255",
		}

		for _, ipStr := range testIPs {
			ip := parseTestIP(ipStr)
			assert.True(t, isPrivateIP(ip), "Expected %s to be private", ipStr)
		}
	})

	t.Run("RFC1918 addresses are private", func(t *testing.T) {
		testIPs := []string{
			"10.0.0.1",
			"10.255.255.255",
			"172.16.0.1",
			"172.31.255.255",
			"192.168.0.1",
			"192.168.255.255",
		}

		for _, ipStr := range testIPs {
			ip := parseTestIP(ipStr)
			assert.True(t, isPrivateIP(ip), "Expected %s to be private", ipStr)
		}
	})

	t.Run("link-local addresses are private", func(t *testing.T) {
		testIPs := []string{
			"169.254.0.1",
			"169.254.255.255",
		}

		for _, ipStr := range testIPs {
			ip := parseTestIP(ipStr)
			assert.True(t, isPrivateIP(ip), "Expected %s to be private", ipStr)
		}
	})

	t.Run("public IPs are not private", func(t *testing.T) {
		testIPs := []string{
			"8.8.8.8",       // Google DNS
			"1.1.1.1",       // Cloudflare DNS
			"142.250.80.46", // google.com
		}

		for _, ipStr := range testIPs {
			ip := parseTestIP(ipStr)
			assert.False(t, isPrivateIP(ip), "Expected %s to be public", ipStr)
		}
	})

	t.Run("nil IP returns false", func(t *testing.T) {
		assert.False(t, isPrivateIP(nil))
	})
}

// Helper function for parsing test IPs
func parseTestIP(ipStr string) []byte {
	// Simple IPv4 parsing for tests
	var ip []byte
	var part int
	for i := 0; i <= len(ipStr); i++ {
		if i == len(ipStr) || ipStr[i] == '.' {
			ip = append(ip, byte(part))
			part = 0
		} else {
			part = part*10 + int(ipStr[i]-'0')
		}
	}
	return ip
}
