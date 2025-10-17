package syncglobalpullsecret

import (
	"bytes"
	"testing"
)

// TestClearPullSecretContent is a simple test function to verify the cleaning works correctly
func TestClearPullSecretContent(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "empty input",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "normal JSON",
			input:    []byte(`{"auths":{"test.registry.com":{"auth":"dGVzdDp0ZXN0"}}}`),
			expected: []byte(`{"auths":{"test.registry.com":{"auth":"dGVzdDp0ZXN0"}}}`),
		},
		{
			name:     "JSON with newlines and tabs",
			input:    []byte("{\n\t\"auths\": {\n\t\t\"test.registry.com\": {\n\t\t\t\"auth\": \"dGVzdDp0ZXN0\"\n\t\t}\n\t}\n}"),
			expected: []byte(`{"auths":{"test.registry.com":{"auth":"dGVzdDp0ZXN0"}}}`),
		},
		{
			name:     "JSON with carriage returns",
			input:    []byte("{\r\n\"auths\": {\r\n\t\"test.registry.com\": {\r\n\t\t\"auth\": \"dGVzdDp0ZXN0\"\r\n\t}\r\n}\r\n}"),
			expected: []byte(`{"auths":{"test.registry.com":{"auth":"dGVzdDp0ZXN0"}}}`),
		},
		{
			name:     "JSON with mixed invisible characters",
			input:    []byte("{\x00\n\t\"auths\": {\x01\n\t\t\"test.registry.com\": {\x02\n\t\t\t\"auth\": \"dGVzdDp0ZXN0\"\x03\n\t\t}\x04\n\t}\x05\n}\x06"),
			expected: []byte(`{"auths":{"test.registry.com":{"auth":"dGVzdDp0ZXN0"}}}`),
		},
		{
			name:     "JSON with spaces (should preserve spaces)",
			input:    []byte(`{"auths":{"test.registry.com":{"auth":"dGVzdDp0ZXN0"}}}`),
			expected: []byte(`{"auths":{"test.registry.com":{"auth":"dGVzdDp0ZXN0"}}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clearPullSecretContent(tt.input)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("clearPullSecretContent() = %q, want %q", string(result), string(tt.expected))
			}
		})
	}
}
