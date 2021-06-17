package machine

import (
	"reflect"
	"testing"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

func TestMisc(t *testing.T) {
	for _, tst := range []struct {
		js   string
		resp interface{}
	}{
		{
			js:   "out(1);",
			resp: 1.0,
		},
		{
			js:   "out(\"a\");",
			resp: "a",
		},
		{
			js:   "const a = 2.0; out(a);",
			resp: 2.0,
		},
		{
			js:   "let a = 3.0; out(a);",
			resp: 3.0,
		},
		{
			js:   "const f = (v) => { out(v); }; f(4.0);",
			resp: 4.0,
		},
		{
			js:   "function f(v) { out(v); }; f(5.0);",
			resp: 5.0,
		},
	} {
		m := New()
		var resp interface{}
		m.Globals["out"] = func(i interface{}) (interface{}, error) {
			resp = i
			return nil, nil
		}
		ast, err := js.Parse(parse.NewInputString(tst.js))
		if err != nil {
			t.Fatal(err)
		}
		if err := m.NewRuntime().Run(ast); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(resp, tst.resp) {
			t.Errorf("got %v, want %v", resp, tst.resp)
		}
	}
}
