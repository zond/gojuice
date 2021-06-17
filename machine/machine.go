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

type NotDeclaredError struct {
	Message string
	Item    interface{}
}

func (n NotDeclaredError) Error() string {
	return n.Message
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

func (r *Runtime) Run(ast *js.AST) error {
	evaluator := &Evaluator{Runtime: r}
	_, err := evaluator.Eval(&ast.BlockStmt)
	return err
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
	case *js.BlockStmt:
		return nil, e.EvalBlockStmt(v)
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
	}
	return nil, NotImplementedError{
		Message: fmt.Sprintf("evaluating %#v not yet implemented", i),
		Item:    i,
	}
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

func (e *Evaluator) EvalEqEqComparison(x, y interface{}) bool {
	return fmt.Sprint(x) == fmt.Sprint(y)
}

func (e *Evaluator) EvalEqEqEqComparison(x, y interface{}) bool {
	refX := reflect.ValueOf(x)
	refY := reflect.ValueOf(y)
	if refX.Kind() != refY.Kind() {
		return false
	}
	if refX.Type() != refY.Type() {
		return false
	}
	switch refX.Kind() {
	case reflect.Bool:
		return refX.Bool() == refY.Bool()
	case reflect.Int:
		return refX.Int() == refY.Int()
	case reflect.Float64:
		return refX.Float() == refY.Float()
	case reflect.Ptr:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.Chan:
		fallthrough
	case reflect.Map:
		fallthrough
	case reflect.Slice:
		return refX.Pointer() == refY.Pointer()
	}
	return reflect.DeepEqual(x, y)
}

func (e *Evaluator) EvalAssignment(x, y interface{}) (interface{}, error) {
	return nil, nil
}

func (e *Evaluator) EvalBinaryExpr(expr *js.BinaryExpr) (interface{}, error) {
	x, err := e.Eval(expr.X)
	if err != nil {
		return nil, err
	}
	y, err := e.Eval(expr.Y)
	if err != nil {
		return nil, err
	}
	switch expr.Op {
	case js.EqToken:
		return e.EvalAssignment(x, y)
	case js.EqEqToken:
		return e.EvalEqEqComparison(x, y), nil
	case js.EqEqEqToken:
		return e.EvalEqEqEqComparison(x, y), nil
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
		return strconv.ParseFloat(string(expr.Data), 64)
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
	args := make([]reflect.Value, len(expr.Args.List))
	for idx := range args {
		evaluated, err := e.Eval(expr.Args.List[idx].Value)
		if err != nil {
			return nil, err
		}
		args[idx] = reflect.ValueOf(evaluated)
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
	out := refCallable.Call(args)
	if !out[0].IsNil() {
		res = out[0].Interface()
	}
	if !out[1].IsNil() {
		err = out[1].Interface().(error)
	}
	return res, err
}

func (e *Evaluator) EvalVar(v *js.Var) (interface{}, error) {
	for scope := e.Runtime.Scope; scope != nil; scope = scope.Parent {
		if binding := scope.Get(string(v.Data)); binding != nil {
			return binding.Item, nil
		}
	}
	if item, found := e.Runtime.Globals[string(v.Data)]; found {
		return item, nil
	}
	if item, found := e.Runtime.M.Globals[string(v.Data)]; found {
		return item, nil
	}
	return nil, NotDeclaredError{
		Message: fmt.Sprintf("%#v is not declared", v),
		Item:    v,
	}
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

func (e *Evaluator) EvalBlockStmt(stmt *js.BlockStmt) error {
	e.Runtime.Scope = scope.New(e.Runtime.Scope)
	defer func() {
		e.Runtime.Scope = e.Runtime.Scope.Parent
	}()
	for _, i := range stmt.List {
		if _, err := e.Eval(i); err != nil {
			return err
		}
	}
	return nil
}
