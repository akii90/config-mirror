package mirror

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestBuildMirroredFromValue_Short(t *testing.T) {
	val := BuildMirroredFromValue("default", "my-secret")
	want := "default-my-secret"
	if val != want {
		t.Errorf("got %q, want %q", val, want)
	}
}

func TestBuildMirroredFromValue_AllLowercase(t *testing.T) {
	val := BuildMirroredFromValue("MyNS", "MySecret")
	want := "myns-mysecret"
	if val != want {
		t.Errorf("got %q, want %q", val, want)
	}
}

func TestBuildMirroredFromValue_InvalidChars(t *testing.T) {
	// Slashes and other invalid chars should be replaced with hyphens.
	val := BuildMirroredFromValue("ns/sub", "name@v1")
	// "ns/sub-name@v1" → sanitize → "ns-sub-name-v1"
	if strings.ContainsAny(val, "/@ ") {
		t.Errorf("label value contains invalid chars: %q", val)
	}
}

func TestBuildMirroredFromValue_ExactlyMaxLen(t *testing.T) {
	// Construct a value that after sanitization is exactly 63 chars.
	ns := strings.Repeat("a", 30)
	name := strings.Repeat("b", 32) // "aaa..."-"bbb..." = 30+1+32 = 63
	val := BuildMirroredFromValue(ns, name)
	if len(val) > 63 {
		t.Errorf("label value length %d exceeds 63: %q", len(val), val)
	}
}

func TestBuildMirroredFromValue_TooLong(t *testing.T) {
	ns := strings.Repeat("a", 40)
	name := strings.Repeat("b", 40)
	val := BuildMirroredFromValue(ns, name)
	if len(val) > 63 {
		t.Errorf("label value length %d exceeds 63: %q", len(val), val)
	}
	// Must contain the expected hash suffix.
	raw := ns + "-" + name
	hash := sha256.Sum256([]byte(raw))
	hashHex := fmt.Sprintf("%x", hash[:5])
	if !strings.HasSuffix(val, hashHex) {
		t.Errorf("label value %q does not end with expected hash suffix %q", val, hashHex)
	}
}

func TestBuildMirroredFromValue_NoLeadingTrailingSeparator(t *testing.T) {
	val := BuildMirroredFromValue("ns", "name")
	if len(val) == 0 {
		t.Fatal("empty label value")
	}
	if val[0] == '-' || val[0] == '_' || val[0] == '.' {
		t.Errorf("label value starts with separator: %q", val)
	}
	last := val[len(val)-1]
	if last == '-' || last == '_' || last == '.' {
		t.Errorf("label value ends with separator: %q", val)
	}
}
