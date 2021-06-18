package machine

import (
	"reflect"
	"testing"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/zond/gojuice/scope"
)

func TestMisc(t *testing.T) {
	for _, tst := range []struct {
		js           string
		wantResp     interface{}
		wantManyResp []interface{}
		wantErr      error
	}{
		{
			js:       "out(1);",
			wantResp: 1,
		},
		{
			js:       "out(1.0);",
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
			js:       "out({});",
			wantResp: map[string]interface{}{},
		},
		{
			js:       "out({\"1\": 2});",
			wantResp: map[string]interface{}{"1": 2},
		},
		{
			js:       "out({1: 2});",
			wantResp: map[string]interface{}{"1": 2},
		},
		{
			js:       "out({1: 2.0});",
			wantResp: map[string]interface{}{"1": 2.0},
		},
		{
			js:       "out({\"x\": \"y\"});",
			wantResp: map[string]interface{}{"x": "y"},
		},
		{
			js:       "const a = {\"b\": 2.0}; out(a.b);",
			wantResp: 2.0,
		},
		{
			js:       "const a = {\"b\": 2.0}; a.c = 3.0; out(a.c);",
			wantResp: 3.0,
		},
		{
			js:       "let a = 1.0; a = 2.0; out(a);",
			wantResp: 2.0,
		},
		{
			js:      "const a = 1.0; a = 2.0; out(a);",
			wantErr: scope.MutatingConstantError{},
		},
		{
			js:      "const a = 1.0; a.b = 2.0;",
			wantErr: NotObjectError{},
		},
		{
			js: "const a = {\"1\": 2, \"3\": 4}; const b = {}; for (const k in a) { b[k] = a[k]; }; out(b);",
			wantResp: map[string]interface{}{
				"1": 2,
				"3": 4,
			},
		},
		{
			js:       "out([1,2,3]);",
			wantResp: []interface{}{1, 2, 3},
		},
		{
			js:       "const a = [0,2,4]; out(a[1]);",
			wantResp: 2,
		},
		{
			js:       "const a = [0,2,4]; a[1] = 6; out(a[1]);",
			wantResp: 6,
		},
		{
			js:       "const a = {\"x\": \"y\"}; a[\"x\"] = \"z\"; out(a[\"x\"]);",
			wantResp: "z",
		},
		{
			js:           "let a = [3,2,1]; for (let e in a) { out(e); }",
			wantManyResp: []interface{}{3, 2, 1},
		},
		{
			js: "const a = {\"x\": 1, \"y\": 2}; const b = {}; a.forEach((k, v) => { b[k] = v; }); out(b);",
			wantResp: map[string]interface{}{
				"x": 1,
				"y": 2,
			},
		},
		{
			js: "const a = {\"x\": 1, \"y\": 2}; out(a.map((k, v) => { return [v, k]; }));",
			wantResp: map[string]interface{}{
				"1": "x",
				"2": "y",
			},
		},
		{
			js:       "out(1 + 2);",
			wantResp: 3,
		},
		{
			js:       "out(1.0 + 2);",
			wantResp: 3.0,
		},
		{
			js:       "out(1 + 2.0);",
			wantResp: 3.0,
		},
		{
			js:       "out(1.0 + 2.0);",
			wantResp: 3.0,
		},
		{
			js:       "out(\"1 \" + 2.1);",
			wantResp: "1 2.1",
		},
		{
			js:       "out([1,2] + [3,4]);",
			wantResp: []interface{}{1, 2, 3, 4},
		},
		{
			js:       "const a = {\"x\": 1, \"y\": 2}; out(a.reduce((k, v, sum) => { return sum + v; }, 0));",
			wantResp: 3,
		},
		{
			js:       "out([1,2,3].map((v) => { return v * 2; }));",
			wantResp: []interface{}{2, 4, 6},
		},
		{
			js:           "let a = [1,2,3]; a.forEach((el) => { out(el); });",
			wantManyResp: []interface{}{1, 2, 3},
		},
		{
			js:       "let a = [1,2,3]; out(a.reduce((el, sum) => { return sum + el; }, 0));",
			wantResp: 6,
		},
	} {
		m := New()
		resp := []interface{}{}
		m.Globals["out"] = func(i interface{}) (interface{}, error) {
			resp = append(resp, i)
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
		if err == nil {
			if tst.wantResp != nil {
				if len(resp) != 1 {
					t.Errorf("%q produced %v, expected a single value", tst.js, resp)
				}
				if !reflect.DeepEqual(resp[0], tst.wantResp) {
					t.Errorf("%q produced %#v, want single value %#v", tst.js, resp[0], tst.wantResp)
				}
			}
			if tst.wantManyResp != nil {
				if !reflect.DeepEqual(resp, tst.wantManyResp) {
					t.Errorf("%q produced %#v, want multi values %#v", tst.js, resp, tst.wantManyResp)
				}
			}
		}
	}
}
