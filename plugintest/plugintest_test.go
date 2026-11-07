package plugintest_test

import (
	"testing"

	plugin "github.com/yxxbc/mygo-plugin"
	"github.com/yxxbc/mygo-plugin/plugintest"
)

func TestNew(t *testing.T) {
	p := plugintest.New(t, "../testdata/hello", plugin.Options{})
	var out struct {
		Text string `json:"text"`
	}
	if err := p.Call("hello", map[string]string{"name": "test"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Text != "hello test" {
		t.Errorf("text = %q, want %q", out.Text, "hello test")
	}
}
