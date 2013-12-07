package interactive

import (
	"errors"
	"fmt"
	"reflect"

	"go/ast"
)

	//"github.com/sbinet/go-readline/pkg/readline"
func evalIdentExpr(ident *ast.Ident, env *Env) (*reflect.Value, bool, error) {
	name := ident.Name
	if name == "nil" {
		return nil, false, nil
	} else if v, ok := env.Vars[name]; ok {
		elem := v.Elem()
		return &elem, true, nil
	} else if v, ok := env.Consts[name]; ok {
		return &v, false, nil
	} else if v, ok := env.Funcs[name]; ok {
		return &v, true, nil
	} else if v, ok := builtinFuncs[name]; ok {
		return &v, false, nil
	} else if p, ok := env.Pkgs[name]; ok {
		val := reflect.ValueOf(p)
		return &val, true, nil
	} else {
		return nil, false, errors.New(fmt.Sprintf("%s undefined", name))
	}
}
