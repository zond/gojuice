package machine

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/tdewolff/parse/v2/js"
	"github.com/zond/gojuice/scope"
)

var (
	ifaceType = reflect.TypeOf((*interface{})(nil)).Elem()
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

type NotPairError struct {
	Message string
	Item    interface{}
}

func (n NotPairError) Error() string {
	return n.Message
}

type NotFunctionError struct {
	Message string
	Item    interface{}
}

func (n NotFunctionError) Error() string {
	return n.Message
}

type IndexOutOfBoundsError struct {
	Message string
	Item    interface{}
	Index   interface{}
}

func (i IndexOutOfBoundsError) Error() string {
	return i.Message
}

type NonIntegerIndexError struct {
	Message string
	Item    interface{}
	Index   interface{}
}

func (n NonIntegerIndexError) Error() string {
	return n.Message
}

type NotObjectError struct {
	Message string
	Item    interface{}
}

func (n NotObjectError) Error() string {
	return n.Message
}

type NotDeclaredError struct {
	Message string
	Item    interface{}
}

func (n NotDeclaredError) Error() string {
	return n.Message
}

type BinaryOpNotImplementedError struct {
	Message string
	X       interface{}
	Y       interface{}
}

func (b BinaryOpNotImplementedError) Error() string {
	return b.Message
}

type NotImplementedError struct {
	Message string
	Item    interface{}
}

func (n NotImplementedError) Error() string {
	return n.Message
}

type NotCallableError struct {
	Message string
	Item    interface{}
}

func (n NotCallableError) Error() string {
	return n.Message
}

type WrongNumberOfArgsError struct {
	Message string
	Item    interface{}
	Got     int
	Want    int
}

func (w WrongNumberOfArgsError) Error() string {
	return w.Message
}

type WrongReturnValueError struct {
	Message string
	Item    interface{}
	Got     reflect.Type
	Want    reflect.Type
}

func (w WrongReturnValueError) Error() string {
	return w.Message
}

type NoReturnValueError struct {
	Message string
	Item    interface{}
}

func (n NoReturnValueError) Error() string {
	return n.Message
}

type M struct {
	Runtimes []*Runtime
	Globals  map[string]interface{}
	Debug    bool
}

func New() *M {
	return &M{
		Runtimes: nil,
		Globals:  map[string]interface{}{},
	}
}

type Runtime struct {
	M       *M
	Globals map[string]interface{}
	Scope   *scope.S
	Debug   bool
}

func (m *M) NewRuntime() *Runtime {
	r := &Runtime{
		M:       m,
		Globals: map[string]interface{}{},
		Scope:   scope.New(nil),
	}
	m.Runtimes = append(m.Runtimes, r)
	return r
}

func (r *Runtime) Lookup(name string) (interface{}, error) {
	for scope := r.Scope; scope != nil; scope = scope.Parent {
		if binding := scope.Get(name); binding != nil {
			return binding.Item, nil
		}
	}
	if item, found := r.Globals[name]; found {
		return item, nil
	}
	if item, found := r.M.Globals[name]; found {
		return item, nil
	}
	return nil, NotDeclaredError{
		Message: fmt.Sprintf("%q is not declared", name),
		Item:    name,
	}
}

func (r *Runtime) Run(ast *js.AST) error {
	evaluator := &Evaluator{Runtime: r}
	_, err := evaluator.Eval(&ast.BlockStmt)
	return err
}

func Call(callable interface{}, iArgs []interface{}) (interface{}, error) {
	args := make([]reflect.Value, len(iArgs))
	for idx := range args {
		if iArgs[idx] == nil {
			args[idx] = reflect.New(ifaceType).Elem()
		} else {
			args[idx] = reflect.ValueOf(iArgs[idx])
		}
	}
	refCallable := reflect.ValueOf(callable)
	if refCallable.Kind() != reflect.Func {
		return nil, NotCallableError{
			Message: fmt.Sprintf("%#v is not callable", callable),
			Item:    callable,
		}
	}
	refType := reflect.TypeOf(callable)
	if !refType.IsVariadic() && refType.NumIn() != len(args) {
		return nil, WrongNumberOfArgsError{
			Message: fmt.Sprintf("%#v takes %v args, got %v", callable, refType.NumIn(), len(args)),
			Item:    callable,
			Got:     len(args),
			Want:    refType.NumIn(),
		}
	}
	if refType.NumOut() != 2 {
		return nil, NoReturnValueError{
			Message: fmt.Sprintf("%#v doesn't return exactly two values", callable),
			Item:    callable,
		}
	}
	if refType.Out(0) != ifaceType {
		return nil, WrongReturnValueError{
			Message: fmt.Sprintf("%#v doesn't return an empty interface as first value", callable),
			Item:    callable,
			Got:     refType.Out(0),
			Want:    ifaceType,
		}
	}
	if refType.Out(1) != errorType {
		return nil, WrongReturnValueError{
			Message: fmt.Sprintf("%#v doesn't return an error as second value", callable),
			Item:    callable,
			Got:     refType.Out(1),
			Want:    errorType,
		}
	}
	var res interface{}
	var err error
	out := refCallable.Call(args)
	if !out[0].IsNil() {
		res = out[0].Interface()
	}
	if !out[1].IsNil() {
		err = out[1].Interface().(error)
	}
	return res, err
}

func (r *Runtime) Call(funcName string, args ...interface{}) (interface{}, error) {
	f, err := r.Lookup(funcName)
	if err != nil {
		return nil, err
	}
	return Call(f, args)
}

type Evaluator struct {
	Runtime *Runtime
}

func (e *Evaluator) Eval(i interface{}) (interface{}, error) {
	if e.Runtime.Debug || e.Runtime.M.Debug {
		fmt.Printf("Eval(%#v)\n", i)
	}
	if err := e.ThrottleEvaluation(i); err != nil {
		return nil, err
	}
	if i == nil {
		return nil, nil
	}
	switch v := i.(type) {
	case *js.IfStmt:
		return nil, e.EvalIfStmt(v)
	case *js.ReturnStmt:
		return e.EvalReturnStmt(v)
	case *js.BlockStmt:
		return e.EvalBlockStmt(v)
	case *js.ExprStmt:
		return e.Eval(v.Value)
	case *js.VarDecl:
		return nil, e.EvalVarDecl(v)
	case *js.LiteralExpr:
		return e.EvalLiteralExpr(v)
	case *js.CallExpr:
		return e.EvalCallExpr(v)
	case *js.Var:
		return e.EvalVar(v)
	case *js.BinaryExpr:
		return e.EvalBinaryExpr(v)
	case *js.ArrowFunc:
		return e.EvalArrowFunc(v)
	case *js.FuncDecl:
		return nil, e.EvalFuncDecl(v)
	case *js.ObjectExpr:
		return e.EvalObjectExpr(v)
	case *js.ArrayExpr:
		return e.EvalArrayExpr(v)
	case *js.DotExpr:
		return e.EvalDotExpr(v)
	case *js.ForInStmt:
		return nil, e.EvalForInStmt(v)
	case *js.IndexExpr:
		return e.EvalIndexExpr(v)
	}
	return nil, NotImplementedError{
		Message: fmt.Sprintf("evaluating %#v not yet implemented", i),
		Item:    i,
	}
}

func (e *Evaluator) EvalReturnStmt(stmt *js.ReturnStmt) (interface{}, error) {
	return e.Eval(stmt.Value)
}

func (e *Evaluator) EvalIndexExpr(expr *js.IndexExpr) (interface{}, error) {
	x, err := e.Eval(expr.X)
	if err != nil {
		return nil, err
	}
	y, err := e.Eval(expr.Y)
	if err != nil {
		return nil, err
	}
	switch v := x.(type) {
	case map[string]interface{}:
		return v[fmt.Sprint(y)], nil
	case []interface{}:
		switch idx := y.(type) {
		case int:
			if idx < 0 {
				idx = idx % len(v)
			}
			if idx > len(v) {
				return nil, IndexOutOfBoundsError{
					Message: fmt.Sprintf("can only index within length %v of array, not %v", len(v), idx),
					Item:    v,
					Index:   idx,
				}
			}
			return v[idx], nil
		default:
			return nil, NonIntegerIndexError{
				Message: fmt.Sprintf("can only index arrays using integers, not %#v", y),
				Item:    v,
				Index:   y,
			}
		}
	}
	return nil, NotImplementedError{
		Message: fmt.Sprintf("index expression %#v on %#v not yet implemented", expr, x),
		Item:    expr,
	}
}

func (e *Evaluator) EvalArrayExpr(expr *js.ArrayExpr) (interface{}, error) {
	res := make([]interface{}, 0, len(expr.List))
	for _, el := range expr.List {
		v, err := e.Eval(el.Value)
		if err != nil {
			return nil, err
		}
		res = append(res, v)
	}
	return res, nil
}

func (e *Evaluator) EvalForInStmt(stmt *js.ForInStmt) error {
	val, err := e.Eval(stmt.Value)
	if err != nil {
		return err
	}
	switch init := stmt.Init.(type) {
	case *js.VarDecl:
		if len(init.List) != 1 {
			return NotImplementedError{
				Message: fmt.Sprintf("for in statement with init %#v not implemented", init),
				Item:    init,
			}
		}
		iterator := func(el interface{}) error {
			e.Runtime.Scope = scope.New(e.Runtime.Scope)
			return func() error {
				defer func() {
					e.Runtime.Scope = e.Runtime.Scope.Parent
				}()
				if err := e.EvalBindingElement(init.List[0], el, init.TokenType == js.ConstToken); err != nil {
					return err
				}
				_, err := e.Eval(stmt.Body)
				return err
			}()
		}
		switch v := val.(type) {
		case map[string]interface{}:
			for k := range v {
				if err := iterator(k); err != nil {
					return err
				}
			}
			return nil
		case []interface{}:
			for _, el := range v {
				if err := iterator(el); err != nil {
					return err
				}
			}
			return nil
		default:
			return NotImplementedError{
				Message: fmt.Sprintf("for in statement with on %#v not implemented", val),
				Item:    init,
			}
		}
	}
	return NotImplementedError{
		Message: fmt.Sprintf("init clause of for statmement %#v not yet implemented", stmt),
		Item:    stmt,
	}
}

func (e *Evaluator) AssertJSFunc(i interface{}) (func(...interface{}) (interface{}, error), error) {
	f, ok := i.(func(...interface{}) (interface{}, error))
	if !ok {
		return nil, NotFunctionError{
			Message: fmt.Sprintf("%#v isn't a JS function", i),
			Item:    i,
		}
	}
	return f, nil
}

func (e *Evaluator) EvalDotExpr(expr *js.DotExpr) (interface{}, error) {
	x, err := e.Eval(expr.X)
	if err != nil {
		return nil, err
	}
	switch v := x.(type) {
	case map[string]interface{}:
		switch string(expr.Y.Data) {
		case "reduce":
			return func(iIterator, sum interface{}) (interface{}, error) {
				iterator, err := e.AssertJSFunc(iIterator)
				if err != nil {
					return nil, err
				}
				for key, val := range v {
					sum, err = iterator(key, val, sum)
					if err != nil {
						return nil, err
					}
				}
				return sum, nil
			}, nil
		case "map":
			return func(iIterator interface{}) (interface{}, error) {
				iterator, err := e.AssertJSFunc(iIterator)
				if err != nil {
					return nil, err
				}
				res := map[string]interface{}{}
				for key, val := range v {
					mapped, err := iterator(key, val)
					if err != nil {
						return nil, err
					}
					switch ary := mapped.(type) {
					case []interface{}:
						if len(ary) != 2 {
							return nil, NotPairError{
								Message: fmt.Sprintf("%#v isn't a pair of two values", mapped),
								Item:    mapped,
							}
						}
						res[fmt.Sprint(ary[0])] = ary[1]
					default:
						return nil, NotPairError{
							Message: fmt.Sprintf("%#v isn't a pair of two values", mapped),
							Item:    mapped,
						}
					}
				}
				return res, nil
			}, nil
		case "forEach":
			return func(iIterator interface{}) (interface{}, error) {
				iterator, err := e.AssertJSFunc(iIterator)
				if err != nil {
					return nil, err
				}
				for key, val := range v {
					_, err := iterator(key, val)
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}, nil
		default:
			return v[string(expr.Y.Data)], nil
		}
	case []interface{}:
		switch string(expr.Y.Data) {
		case "reduce":
			return func(iIterator, sum interface{}) (interface{}, error) {
				iterator, err := e.AssertJSFunc(iIterator)
				if err != nil {
					return nil, err
				}
				for _, el := range v {
					sum, err = iterator(el, sum)
					if err != nil {
						return nil, err
					}
				}
				return sum, nil
			}, nil
		case "map":
			return func(iIterator interface{}) (interface{}, error) {
				iterator, err := e.AssertJSFunc(iIterator)
				if err != nil {
					return nil, err
				}
				res := make([]interface{}, 0, len(v))
				for _, el := range v {
					mapped, err := iterator(el)
					if err != nil {
						return nil, err
					}
					res = append(res, mapped)
				}
				return res, nil
			}, nil
		case "forEach":
			return func(iIterator interface{}) (interface{}, error) {
				iterator, err := e.AssertJSFunc(iIterator)
				if err != nil {
					return nil, err
				}
				for _, el := range v {
					_, err := iterator(el)
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}, nil
		}
	}
	return nil, NotObjectError{
		Message: fmt.Sprintf("%#v is not an object", x),
		Item:    x,
	}
}

func (e *Evaluator) EvalObjectExpr(expr *js.ObjectExpr) (interface{}, error) {
	res := map[string]interface{}{}
	for _, prop := range expr.List {
		name := string(prop.Name.Literal.Data)
		if prop.Name.Computed != nil {
			iName, err := e.Eval(prop.Name.Computed)
			if err != nil {
				return nil, err
			}
			name = fmt.Sprint(iName)
		}
		value, err := e.Eval(prop.Value)
		if err != nil {
			return nil, err
		}
		res[name] = value
	}
	return res, nil
}

func (e *Evaluator) EvalFuncDecl(f *js.FuncDecl) error {
	genF, err := e.GenerateJSFunction(&f.Body, f.Params)
	if err != nil {
		return err
	}
	e.Runtime.Scope.Set(string(f.Name.Data), &scope.Binding{
		Item:     genF,
		Constant: true,
	})
	return nil
}

func (e *Evaluator) GenerateJSFunction(body *js.BlockStmt, expectedParams js.Params) (interface{}, error) {
	parentScope := e.Runtime.Scope
	return func(actualParams ...interface{}) (interface{}, error) {
		currentScope := e.Runtime.Scope
		e.Runtime.Scope = scope.New(parentScope)
		defer func() {
			e.Runtime.Scope = currentScope
		}()
		if len(actualParams) > len(expectedParams.List) {
			return nil, WrongNumberOfArgsError{
				Message: fmt.Sprintf("%#v takes %v args, got %v", body, len(expectedParams.List), len(actualParams)),
				Item:    body,
				Got:     len(actualParams),
				Want:    len(expectedParams.List),
			}
		}
		for idx, el := range expectedParams.List {
			var value interface{}
			if idx < len(actualParams) {
				value = actualParams[idx]
			}
			if err := e.EvalBindingElement(el, value, false); err != nil {
				return nil, err
			}
		}
		return e.Eval(body)
	}, nil
}

func (e *Evaluator) EvalArrowFunc(f *js.ArrowFunc) (interface{}, error) {
	return e.GenerateJSFunction(&f.Body, f.Params)
}

func EqEqComparison(x, y interface{}) (bool, error) {
	return fmt.Sprint(x) == fmt.Sprint(y), nil
}

func EqEqEqComparison(x, y interface{}) (bool, error) {
	refX := reflect.ValueOf(x)
	refY := reflect.ValueOf(y)
	if refX.Kind() != refY.Kind() {
		return false, nil
	}
	if refX.Type() != refY.Type() {
		return false, nil
	}
	switch refX.Kind() {
	case reflect.Bool:
		return refX.Bool() == refY.Bool(), nil
	case reflect.Int:
		return refX.Int() == refY.Int(), nil
	case reflect.Float64:
		return refX.Float() == refY.Float(), nil
	case reflect.Ptr:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.Chan:
		fallthrough
	case reflect.Map:
		fallthrough
	case reflect.Slice:
		return refX.Pointer() == refY.Pointer(), nil
	}
	return reflect.DeepEqual(x, y), nil
}

func (e *Evaluator) EvalAssignment(expr *js.BinaryExpr) (interface{}, error) {
	y, err := e.Eval(expr.Y)
	if err != nil {
		return nil, err
	}
	switch v := expr.X.(type) {
	case *js.Var:
		if err := e.Runtime.Scope.Set(string(v.Data), &scope.Binding{
			Item:     y,
			Constant: false,
		}); err != nil {
			return nil, err
		}
		return y, nil
	case *js.DotExpr:
		obj, err := e.Eval(v.X)
		if err != nil {
			return nil, err
		}
		switch hmap := obj.(type) {
		case map[string]interface{}:
			hmap[string(v.Y.Data)] = y
			return y, nil
		default:
			return nil, NotObjectError{
				Message: fmt.Sprintf("%#v is not an object", obj),
				Item:    obj,
			}
		}
	case *js.IndexExpr:
		obj, err := e.Eval(v.X)
		if err != nil {
			return nil, err
		}
		idx, err := e.Eval(v.Y)
		if err != nil {
			return nil, err
		}
		switch ass := obj.(type) {
		case map[string]interface{}:
			ass[fmt.Sprint(idx)] = y
			return y, nil
		case []interface{}:
			switch i := idx.(type) {
			case int:
				if i < 0 {
					i = i % len(ass)
				}
				if i+1 > len(ass) {
					return nil, IndexOutOfBoundsError{
						Message: fmt.Sprintf("can only index within length %v of array, not %v", len(ass), i),
						Item:    ass,
						Index:   i,
					}
				}
				ass[i] = y
				return y, nil
			default:
				return nil, NonIntegerIndexError{
					Message: fmt.Sprintf("can only index arrays using integers, not %#v", idx),
					Item:    ass,
					Index:   idx,
				}
			}
		default:
			return nil, NotObjectError{
				Message: fmt.Sprintf("#%v is not an object or an array", obj),
				Item:    obj,
			}
		}
	}
	return nil, NotImplementedError{
		Message: fmt.Sprintf("assignment to %#v not yet implemented", expr.X),
		Item:    expr.X,
	}
}

func Add(x, y interface{}) (interface{}, error) {
	switch xv := x.(type) {
	case int:
		switch yv := y.(type) {
		case int:
			return xv + yv, nil
		case float64:
			return float64(xv) + yv, nil
		}
	case float64:
		switch yv := y.(type) {
		case int:
			return xv + float64(yv), nil
		case float64:
			return xv + yv, nil
		}
	case string:
		switch yv := y.(type) {
		case int:
			return xv + fmt.Sprint(yv), nil
		case float64:
			return xv + fmt.Sprint(yv), nil
		case string:
			return xv + fmt.Sprint(yv), nil
		}
	case []interface{}:
		switch yv := y.(type) {
		case []interface{}:
			res := make([]interface{}, len(xv)+len(yv))
			copy(res, xv)
			copy(res[len(xv):], yv)
			return res, nil
		}
	}
	return nil, BinaryOpNotImplementedError{
		Message: fmt.Sprintf("add of %#v and %#v not implemented", x, y),
		X:       x,
		Y:       y,
	}
}

func Div(x, y interface{}) (interface{}, error) {
	switch xv := x.(type) {
	case int:
		switch yv := y.(type) {
		case int:
			return xv / yv, nil
		case float64:
			return float64(xv) / yv, nil
		}
	case float64:
		switch yv := y.(type) {
		case int:
			return xv / float64(yv), nil
		case float64:
			return xv / yv, nil
		}
	}
	return nil, BinaryOpNotImplementedError{
		Message: fmt.Sprintf("div of %#v and %#v not implemented", x, y),
		X:       x,
		Y:       y,
	}
}

func Sub(x, y interface{}) (interface{}, error) {
	switch xv := x.(type) {
	case int:
		switch yv := y.(type) {
		case int:
			return xv - yv, nil
		case float64:
			return float64(xv) - yv, nil
		}
	case float64:
		switch yv := y.(type) {
		case int:
			return xv - float64(yv), nil
		case float64:
			return xv - yv, nil
		}
	}
	return nil, BinaryOpNotImplementedError{
		Message: fmt.Sprintf("sub of %#v and %#v not implemented", x, y),
		X:       x,
		Y:       y,
	}
}

func Mul(x, y interface{}) (interface{}, error) {
	switch xv := x.(type) {
	case int:
		switch yv := y.(type) {
		case int:
			return xv * yv, nil
		case float64:
			return float64(xv) * yv, nil
		}
	case float64:
		switch yv := y.(type) {
		case int:
			return xv * float64(yv), nil
		case float64:
			return xv * yv, nil
		}
	case string:
		switch yv := y.(type) {
		case int:
			res := ""
			for i := 0; i < yv; i++ {
				res += xv
			}
			return res, nil
		}
	case []interface{}:
		switch yv := y.(type) {
		case int:
			res := make([]interface{}, len(xv)*yv)
			for i := 0; i < yv; i++ {
				copy(res[i*len(xv):], xv)
			}
			return res, nil
		}
	}
	return nil, BinaryOpNotImplementedError{
		Message: fmt.Sprintf("mul of %#v and %#v not implemented", x, y),
		X:       x,
		Y:       y,
	}
}

func (e *Evaluator) EvalBinaryExpr(expr *js.BinaryExpr) (interface{}, error) {
	if expr.Op == js.EqToken {
		return e.EvalAssignment(expr)
	}
	x, err := e.Eval(expr.X)
	if err != nil {
		return nil, err
	}
	y, err := e.Eval(expr.Y)
	if err != nil {
		return nil, err
	}
	switch expr.Op {
	case js.EqEqToken:
		return EqEqComparison(x, y)
	case js.EqEqEqToken:
		return EqEqEqComparison(x, y)
	case js.AddToken:
		return Add(x, y)
	case js.SubToken:
		return Sub(x, y)
	case js.MulToken:
		return Mul(x, y)
	}
	return nil, NotImplementedError{
		Message: fmt.Sprintf("evaluating binary expression %#v not yet implemented", expr),
		Item:    expr,
	}
}

func (e *Evaluator) EvalTruth(iVal interface{}) bool {
	if iVal == nil {
		return false
	}
	switch val := iVal.(type) {
	case float64:
		return val != 0.0
	case int:
		return val != 0
	case string:
		return val != ""
	case bool:
		return val
	default:
		refVal := reflect.ValueOf(iVal)
		switch refVal.Kind() {
		case reflect.Chan:
			fallthrough
		case reflect.Func:
			fallthrough
		case reflect.Interface:
			fallthrough
		case reflect.Map:
			fallthrough
		case reflect.Ptr:
			fallthrough
		case reflect.Slice:
			return !refVal.IsNil()
		}
	}
	return true
}

func (e *Evaluator) EvalIfStmt(stmt *js.IfStmt) error {
	cond, err := e.Eval(stmt.Cond)
	if err != nil {
		return err
	}
	if e.EvalTruth(cond) {
		_, err = e.Eval(stmt.Body)
	} else {
		_, err = e.Eval(stmt.Else)
	}
	return err
}

func (e *Evaluator) EvalLiteralExpr(expr *js.LiteralExpr) (interface{}, error) {
	switch expr.TokenType {
	case js.DecimalToken:
		intVal, err := strconv.Atoi(string(expr.Data))
		if err != nil {
			return strconv.ParseFloat(string(expr.Data), 64)
		}
		return intVal, nil
	case js.StringToken:
		return string(expr.Data[1 : len(expr.Data)-1]), nil
	}
	return nil, NotImplementedError{
		Message: fmt.Sprintf("evaluating literal %#v not yet implemented", expr),
		Item:    expr,
	}
}

func (e *Evaluator) EvalCallExpr(expr *js.CallExpr) (interface{}, error) {
	callable, err := e.Eval(expr.X)
	if err != nil {
		return nil, err
	}
	args := make([]interface{}, len(expr.Args.List))
	for idx := range args {
		args[idx], err = e.Eval(expr.Args.List[idx].Value)
		if err != nil {
			return nil, err
		}
	}
	return Call(callable, args)
}

func (e *Evaluator) EvalVar(v *js.Var) (interface{}, error) {
	return e.Runtime.Lookup(string(v.Data))
}

func (e *Evaluator) ThrottleEvaluation(i interface{}) error {
	return nil
}

func (e *Evaluator) ThrottleAllocation(i interface{}) error {
	return nil
}

func (e *Evaluator) EvalBindingElement(el js.BindingElement, value interface{}, constant bool) error {
	if value == nil {
		var err error
		value, err = e.Eval(el.Default)
		if err != nil {
			return err
		}
	}
	if err := e.ThrottleAllocation(value); err != nil {
		return err
	}
	switch bind := el.Binding.(type) {
	case *js.Var:
		e.Runtime.Scope.Set(string(bind.Data), &scope.Binding{
			Item:     value,
			Constant: constant,
		})
		return nil
	}
	return NotImplementedError{
		Message: fmt.Sprintf("evaluating binding element %#v not yet implemented", el),
		Item:    el,
	}
}

func (e *Evaluator) EvalVarDecl(varDecl *js.VarDecl) error {
	for _, el := range varDecl.List {
		if err := e.EvalBindingElement(el, nil, varDecl.TokenType == js.ConstToken); err != nil {
			return err
		}
	}
	return nil
}

func (e *Evaluator) EvalBlockStmt(stmt *js.BlockStmt) (interface{}, error) {
	e.Runtime.Scope = scope.New(e.Runtime.Scope)
	defer func() {
		e.Runtime.Scope = e.Runtime.Scope.Parent
	}()
	var res interface{}
	var err error
	for _, i := range stmt.List {
		if res, err = e.Eval(i); err != nil {
			return nil, err
		}
	}
	return res, nil
}
