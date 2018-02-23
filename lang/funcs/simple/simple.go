// Mgmt
// Copyright (C) 2013-2018+ James Shubin and the project contributors
// Written by James Shubin <james@shubin.ca> and the project contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package simple

import (
	"fmt"

	"github.com/purpleidea/mgmt/lang/funcs"
	"github.com/purpleidea/mgmt/lang/interfaces"
	"github.com/purpleidea/mgmt/lang/types"

	errwrap "github.com/pkg/errors"
)

// RegisteredFuncs maps a function name to the corresponding static, pure func.
var RegisteredFuncs = make(map[string]*types.FuncValue) // must initialize

// Register registers a simple, static, pure function. It is easier to use than
// the raw function API, but also limits you to simple, static, pure functions.
func Register(name string, fn *types.FuncValue) {
	if _, exists := RegisteredFuncs[name]; exists {
		panic(fmt.Sprintf("a simple func named %s is already registered", name))
	}
	RegisteredFuncs[name] = fn // store a copy for ourselves

	// register a copy in the main function database
	funcs.Register(name, func() interfaces.Func { return &simpleFunc{Fn: fn} })
}

// simpleFunc is a scaffolding function struct which fulfills the boiler-plate
// for the function API, but that can run a very simple, static, pure function.
type simpleFunc struct {
	Fn *types.FuncValue

	init *interfaces.Init
	last types.Value // last value received to use for diff

	result types.Value // last calculated output

	closeChan chan struct{}
}

// Validate makes sure we've built our struct properly. It is usually unused for
// normal functions that users can use directly.
func (obj *simpleFunc) Validate() error {
	if obj.Fn == nil { // build must be run first
		return fmt.Errorf("type is still unspecified")
	}
	return nil
}

// Info returns some static info about itself.
func (obj *simpleFunc) Info() *interfaces.Info {
	return &interfaces.Info{
		Pure: true,
		Memo: false, // TODO: should this be something we specify here?
		Sig:  obj.Fn.Type(),
		Err:  obj.Validate(),
	}
}

// Init runs some startup code for this function.
func (obj *simpleFunc) Init(init *interfaces.Init) error {
	obj.init = init
	obj.closeChan = make(chan struct{})
	return nil
}

// Stream returns the changing values that this func has over time.
func (obj *simpleFunc) Stream() error {
	defer close(obj.init.Output) // the sender closes
	for {
		select {
		case input, ok := <-obj.init.Input:
			if !ok {
				if len(obj.Fn.Type().Ord) > 0 {
					return nil // can't output any more
				}
				// no inputs were expected, pass through once
			}
			if ok {
				//if err := input.Type().Cmp(obj.Info().Sig.Input); err != nil {
				//	return errwrap.Wrapf(err, "wrong function input")
				//}

				if obj.last != nil && input.Cmp(obj.last) == nil {
					continue // value didn't change, skip it
				}
				obj.last = input // store for next
			}

			values := []types.Value{}
			for _, name := range obj.Fn.Type().Ord {
				x := input.Struct()[name]
				values = append(values, x)
			}

			result, err := obj.Fn.Call(values) // (Value, error)
			if err != nil {
				return errwrap.Wrapf(err, "simple function errored")
			}

			if obj.result == result {
				continue // result didn't change
			}
			obj.result = result // store new result

		case <-obj.closeChan:
			return nil
		}

		select {
		case obj.init.Output <- obj.result: // send
			if len(obj.Fn.Type().Ord) == 0 {
				return nil // no more values, we're a pure func
			}
		case <-obj.closeChan:
			return nil
		}
	}
}

// Close runs some shutdown code for this function and turns off the stream.
func (obj *simpleFunc) Close() error {
	close(obj.closeChan)
	return nil
}
