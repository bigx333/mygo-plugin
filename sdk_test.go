package plugin

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestHostTypeScript(t *testing.T) {
	got, err := os.ReadFile("spec/host.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != HostTypeScript {
		t.Errorf("spec/host.d.ts and HostTypeScript differ at %s", firstDifference(string(got), HostTypeScript))
	}
	for _, mention := range []string{"declare const host", "storage", "aesEcbEncrypt"} {
		if !strings.Contains(HostTypeScript, mention) {
			t.Errorf("HostTypeScript does not mention %q", mention)
		}
	}
	if HostTypeScriptVersion != 1 {
		t.Errorf("HostTypeScriptVersion = %d, want 1", HostTypeScriptVersion)
	}
}

// firstDifference says where two texts part company, for a test that keeps two
// copies of one declaration together.
func firstDifference(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) && i < len(lb); i++ {
		if la[i] != lb[i] {
			return fmt.Sprintf("line %d: %q is not %q", i+1, la[i], lb[i])
		}
	}
	return fmt.Sprintf("%d lines against %d", len(la), len(lb))
}
