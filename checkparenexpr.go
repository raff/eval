package eval

import (
	"go/ast"
)

func checkParenExpr(ctx *Ctx, paren *ast.ParenExpr, env *Env) (aexpr *ParenExpr, errs []error) {
	aexpr = &ParenExpr{ParenExpr: paren}

	var moreErrs []error
	if aexpr.X, moreErrs = CheckExpr(ctx, paren.X, env); moreErrs != nil {
		errs = append(errs, moreErrs...)
	}
	return aexpr, errs
}
