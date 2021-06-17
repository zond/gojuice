package main

import (
	"flag"
	"fmt"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/zond/gojuice/machine"
)

func main() {
	input := flag.String("input", "", "What to run")
	debug := flag.Bool("debug", false, "Whether to log all evaluations")
	flag.Parse()
	ast, err := js.Parse(parse.NewInputString(*input))
	if err != nil {
		panic(err)
	}
	m := machine.New()
	m.Debug = *debug
	m.Globals["log"] = func(params ...interface{}) (interface{}, error) {
		fmt.Println(params...)
		return nil, nil
	}
	r := m.NewRuntime()
	if err := r.Run(ast); err != nil {
		panic(err)
	}
}
