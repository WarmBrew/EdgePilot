package audit

import "testing"

func TestMaskPassword(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"short password", "abc", "***"},
		{"exact min length", "abcdefg", "*******"},
		{"normal password", "MyStr0ng!Pass", "MySt*********"},
		{"long password", "VeryLongPassword123!", "Very****************"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskPassword(tt.input)
			if result != tt.expected {
				t.Errorf("MaskPassword(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty token", "", ""},
		{"short token", "abc", "***"},
		{"normal token", "tok_abc123xyz456", "tok***...z456"},
		{"jwt-like token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test", "eyJ***...test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskToken(tt.input)
			if result != tt.expected {
				t.Errorf("MaskToken(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskFileContent(t *testing.T) {
	tests := []struct {
		name   string
		size   int
		expect string
	}{
		{"zero bytes", 0, "[binary content - 0 bytes]"},
		{"small file", 100, "[binary content - 100 bytes]"},
		{"large file", 1234567, "[binary content - 1234567 bytes]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskFileContent(tt.size)
			if result != tt.expect {
				t.Errorf("MaskFileContent(%d) = %q, want %q", tt.size, result, tt.expect)
			}
		})
	}
}

func TestMaskIPAddr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty ip", "", ""},
		{"valid ipv4", "192.168.1.100", "192.168.*.*"},
		{"localhost", "127.0.0.1", "127.0.*.*"},
		{"ipv6 simple", "2001:db8::1", "2001:***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskIPAddr(tt.input)
			if result != tt.expected {
				t.Errorf("MaskIPAddr(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty email", "", ""},
		{"valid email", "admin@example.com", "a****@example.com"},
		{"single char local", "x@test.com", "x***@test.com"},
		{"no at sign", "invalid", "*******"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskEmail(tt.input)
			if result != tt.expected {
				t.Errorf("MaskEmail(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"single char", "a", "*"},
		{"normal", "secret", "******"},
		{"long", "this is a secret", "****************"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskString(tt.input)
			if result != tt.expected {
				t.Errorf("MaskString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
			if len(result) != len(tt.input) {
				t.Errorf("MaskString(%q) length = %d, want %d", tt.input, len(result), len(tt.input))
			}
		})
	}
}
