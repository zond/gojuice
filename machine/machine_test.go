package machine

import (
	"reflect"
	"testing"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

func TestMisc(t *testing.T) {
	for _, tst := range []struct {
		js       string
		wantResp interface{}
		wantErr  error
	}{
		{
			js:       "out(1);",
			wantResp: 1.0,
		},
		{
			js:       "out(\"a\");",
			wantResp: "a",
		},
		{
			js:       "const a = 2.0; out(a);",
			wantResp: 2.0,
		},
		{
			js:       "let a = 3.0; out(a);",
			wantResp: 3.0,
		},
		{
			js:       "const f = (v) => { out(v); }; f(4.0);",
			wantResp: 4.0,
		},
		{
			js:       "function f(v) { out(v); }; f(5.0);",
			wantResp: 5.0,
		},
		{
			js:       "class A { do(v) { out(v); } }; const a = new A(); a.do(6.0);",
			wantResp: 6.0,
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
			t.Error(err)
			continue
		}
		err = m.NewRuntime().Run(ast)
		if err != nil && tst.wantErr == nil {
			t.Errorf("%q produced %v", tst.js, err)
			continue
		}
		if (err == nil && tst.wantErr != nil) || (reflect.TypeOf(tst.wantErr) != reflect.TypeOf(err)) {
			t.Errorf("%q produced %v, wanted %v", tst.js, err, tst.wantErr)
			continue
		}
		if !reflect.DeepEqual(resp, tst.wantResp) {
			t.Errorf("%q produced %v, want %v", tst.js, resp, tst.wantResp)
		}
	}
}
