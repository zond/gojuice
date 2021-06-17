package scope

import "fmt"

type Binding struct {
	Item     interface{}
	Constant bool
}

type S struct {
	Parent *S

	bindings map[string]*Binding
}

func New(parent *S) *S {
	return &S{
		Parent:   parent,
		bindings: map[string]*Binding{},
	}
}

type MutatingConstantError struct {
	Message string
	Item    interface{}
}

func (m MutatingConstantError) Error() string {
	return m.Message
}

func (s *S) Set(name string, binding *Binding) error {
	if old, found := s.bindings[name]; found && old.Constant {
		return MutatingConstantError{
			Message: fmt.Sprintf("%q => %#v is constant and can't be mutated", name, old),
			Item:    old,
		}
	}
	s.bindings[name] = binding
	return nil
}

func (s *S) Get(name string) *Binding {
	return s.bindings[name]
}
