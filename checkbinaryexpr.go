package interactive

import (
	"fmt"
	"reflect"

	"go/ast"
	"go/token"
)

func checkBinaryExpr(ctx *Ctx, binary *ast.BinaryExpr, env *Env) (aexpr *BinaryExpr, errs []error) {
	aexpr = &BinaryExpr{BinaryExpr: binary}

	var moreErrs []error
	if aexpr.X, moreErrs = CheckExpr(ctx, binary.X, env); moreErrs != nil {
		errs = append(errs, moreErrs...)
	}
	if aexpr.Y, moreErrs = CheckExpr(ctx, binary.Y, env); moreErrs != nil {
		errs = append(errs, moreErrs...)
	}

	/*
	if errs != nil {
		return aexpr, errs
	}
	*/

	xa := aexpr.X.(Expr)
	ya := aexpr.Y.(Expr)

	xt := xa.KnownType()
	yt := ya.KnownType()

	// Check for multi valued expressions. Not much we can do if we find one
	// TODO check for single values

	// Check for compatible types

	// TODO tx and ty will always have a known type once checker is complete
	//      This if() is a shim
	if len(xt) != 1 || len(yt) != 1 {
		return aexpr, errs
	}

	xc, xuntyped := xt[0].(ConstType)
	yc, yuntyped := yt[0].(ConstType)
	if xa.IsConst() && ya.IsConst() {
		if xuntyped && yuntyped {
			yv := ya.Const()
			xv := xa.Const()
			var promoted ConstType
			if promoted, moreErrs = promoteConsts(ctx, xc, yc, xa, ya, xv, yv); moreErrs != nil {
				errs = append(errs, moreErrs...)
				errs = append(errs, ErrInvalidBinaryOperation{at(ctx, aexpr)})
			} else {
				if isBooleanOp(binary.Op) {
					aexpr.knownType = []reflect.Type{ConstBool}
				} else {
					aexpr.knownType = knownType{promoted}
				}
				aexpr.constValue, moreErrs = evalConstUntypedBinaryExpr(ctx, aexpr, promoted)
				if moreErrs != nil {
					errs = append(errs, moreErrs...)
				}
			}
		} else if yuntyped {
			z, moreErrs := evalConstTypedUntypedBinaryExpr(ctx, aexpr, xa, ya, true)
			if moreErrs != nil {
				errs = append(errs, moreErrs...)
			} else {
				aexpr.knownType = knownType{reflect.Value(z).Type()}
				aexpr.constValue = z
			}
		} else if xuntyped {
			z, moreErrs := evalConstTypedUntypedBinaryExpr(ctx, aexpr, ya, xa, false)
			if moreErrs != nil {
				errs = append(errs, moreErrs...)
			} else {
				aexpr.knownType = knownType{reflect.Value(z).Type()}
				aexpr.constValue = z
			}
		} else {
			if z, moreErrs := evalConstTypedBinaryExpr(ctx, aexpr, xa, ya); moreErrs != nil {
				errs = append(errs, moreErrs...)
			} else {
				aexpr.knownType = knownType{reflect.Value(z).Type()}
				aexpr.constValue = z
			}
		}
	}
	return aexpr, errs
}

// Evaluates a const binary Expr. May return a sensical constValue
// even if ErrTruncatedConst errors are present
func evalConstUntypedBinaryExpr(ctx *Ctx, constExpr *BinaryExpr, promotedType ConstType) (constValue, []error) {
	x := constExpr.X.(Expr).Const()
	y := constExpr.Y.(Expr).Const()
	switch promotedType.(type) {
	case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
		xx := x.Interface().(*ConstNumber)
		yy := y.Interface().(*ConstNumber)
		return evalConstBinaryNumericExpr(ctx, constExpr, xx, yy)
	case ConstStringType:
		xx := x.String()
		yy := y.String()
		return evalConstBinaryStringExpr(ctx, constExpr, xx, yy)
	case ConstBoolType:
		xx := x.Bool()
		yy := y.Bool()
		return evalConstBinaryBoolExpr(ctx, constExpr, xx, yy)
	default:
		// It is possible that both x and y are ConstNil, however no operator is defined, not even ==
		return constValue{}, []error{ErrInvalidBinaryOperation{at(ctx, constExpr)}}
	}

}

func evalConstBinaryNumericExpr(ctx *Ctx, constExpr *BinaryExpr, x, y *ConstNumber) (constValue, []error) {
	var errs []error

	switch constExpr.Op {
	case token.ADD:
		return constValueOf(new(ConstNumber).Add(x, y)), nil
	case token.SUB:
		return constValueOf(new(ConstNumber).Sub(x, y)), nil
	case token.MUL:
		return constValueOf(new(ConstNumber).Mul(x, y)), nil
	case token.QUO:
		if y.Value.IsZero() {
			return constValue{}, []error{ErrDivideByZero{at(ctx, constExpr.Y)}}
		}
		return constValueOf(new(ConstNumber).Quo(x, y)), nil
	case token.REM:
		if y.Value.IsZero() {
			return constValue{}, []error{ErrDivideByZero{at(ctx, constExpr.Y)}}
		} else if !(x.Type.IsIntegral() && y.Type.IsIntegral()) {
			return constValue{}, []error{ErrInvalidBinaryOperation{at(ctx, constExpr)}}
		} else {
			return constValueOf(new(ConstNumber).Rem(x, y)), nil
		}
	case token.AND, token.OR, token.XOR, token.AND_NOT:
		if !(x.Type.IsIntegral() && y.Type.IsIntegral()) {
			return constValue{}, []error{ErrInvalidBinaryOperation{at(ctx, constExpr)}}
		}

		switch constExpr.Op {
		case token.AND:
			return constValueOf(new(ConstNumber).And(x, y)), nil
		case token.OR:
			return constValueOf(new(ConstNumber).Or(x, y)), nil
		case token.XOR:
			return constValueOf(new(ConstNumber).Xor(x, y)), nil
		case token.AND_NOT:
			return constValueOf(new(ConstNumber).AndNot(x, y)), nil
		default:
			panic("go-interactive: impossible")
		}

	case token.EQL:
		return constValueOf(x.Value.Equals(&y.Value)), nil
	case token.NEQ:
		return constValueOf(!x.Value.Equals(&y.Value)), nil

	case token.LEQ, token.GEQ, token.LSS, token.GTR:
		var b bool
		if !(x.Type.IsReal() && y.Type.IsReal()) {
			return constValue{}, []error{ErrInvalidBinaryOperation{at(ctx, constExpr)}}
		}
		cmp := x.Value.Re.Cmp(&y.Value.Re)
		switch constExpr.Op {
		case token.NEQ:
			b = cmp != 0
		case token.LEQ:
			b = cmp <= 0
		case token.GEQ:
			b = cmp >= 0
		case token.LSS:
			b = cmp < 0
		case token.GTR:
			b = cmp > 0
		}
		return constValueOf(b), errs
	default:
		return constValue{}, []error{ErrInvalidBinaryOperation{at(ctx, constExpr)}}
	}
}

func evalConstBinaryStringExpr(ctx *Ctx, constExpr *BinaryExpr, x, y string) (constValue, []error) {
	switch constExpr.Op {
	case token.ADD:
		return constValueOf(x + y), nil
	case token.EQL:
		return constValueOf(x == y), nil
	case token.NEQ:
		return constValueOf(x != y), nil
	case token.LEQ:
		return constValueOf(x <= y), nil
	case token.GEQ:
		return constValueOf(x >= y), nil
	case token.LSS:
		return constValueOf(x < y), nil
	case token.GTR:
		return constValueOf(x > y), nil
	default:
		return constValue{}, []error{ErrInvalidBinaryOperation{at(ctx, constExpr)}}
	}
}

func evalConstBinaryBoolExpr(ctx *Ctx, constExpr *BinaryExpr, x, y bool) (constValue, []error) {
	switch constExpr.Op {
	case token.EQL:
		return constValueOf(x == y), nil
	case token.NEQ:
		return constValueOf(x != y), nil
	case token. LAND:
		return constValueOf(x && y), nil
	case token.LOR:
		return constValueOf(x || y), nil
	default:
		return constValue{}, []error{ErrInvalidBinaryOperation{at(ctx, constExpr)}}
	}
}

// Evaluate x op y
func evalConstTypedUntypedBinaryExpr(ctx *Ctx, binary *BinaryExpr, typedExpr, untypedExpr Expr, reversed bool) (
	constValue, []error) {

	xt := untypedExpr.KnownType()[0].(ConstType)
	yt := typedExpr.KnownType()[0]

	// x must be convertible to target type
	xUntyped := untypedExpr.Const()

	x, xConvErrs := convertConstToTyped(ctx, xt, constValue(xUntyped), yt, untypedExpr)

	if !isOpDefinedOn(binary.Op, yt) {
		return constValue{}, append(xConvErrs, ErrInvalidBinaryOperation{at(ctx, binary)})
	}

	switch xt.(type) {
	case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
		xx, xok := convertTypedToConstNumber(reflect.Value(x))
		yy, yok := convertTypedToConstNumber(typedExpr.Const())

		// If a child node errored, then it is possible that, typedExpr.Const() is
		// actually a *ConstNumber to avoid loss of precision in error messages.
		if !yok {
			yy, yok = reflect.Value(typedExpr.Const()).Interface().(*ConstNumber)
		}

		if !xok || !yok {
			// This is a non numeric expression. Return the errors encountered so far
			fmt.Printf("%v\n", xConvErrs)
			return constValue{}, append(xConvErrs, ErrInvalidBinaryOperation{at(ctx, binary)})
		}

		if reversed {
			xx, yy = yy, xx
		}

		z, errs := evalConstBinaryNumericExpr(ctx, binary, xx, yy)
		if errs != nil {
			return constValueOf(z), append(xConvErrs, errs...)
		}
		errs = append(xConvErrs, errs...)

		var zt ConstType
		var rt reflect.Type
		if isBooleanOp(binary.Op) {
			zt = ConstBool
			rt = reflect.TypeOf(false)
		} else {
			zt = reflect.Value(z).Interface().(*ConstNumber).Type
			rt = yt
		}

		r, moreErrs := convertConstToTyped(ctx, zt, z, rt, binary)
		return constValue(r), append(errs, moreErrs...)

	case ConstStringType:
		if yt.Kind() == reflect.String {
			xstring := reflect.Value(x).String()
			ystring := typedExpr.Const().String()

			if reversed {
				xstring, ystring = ystring, xstring
			}

			z, errs := evalConstBinaryStringExpr(ctx, binary, xstring, ystring)
			r, moreErrs := convertConstToTyped(ctx, ConstString, z, yt, untypedExpr)
			return constValue(r), append(errs, moreErrs...)
		}

	case ConstBoolType:
		if yt.Kind() == reflect.Bool {
			xbool := reflect.Value(x).Bool()
			ybool := typedExpr.Const().Bool()

			if reversed {
				xbool, ybool = ybool, xbool
			}

			z, errs := evalConstBinaryBoolExpr(ctx, binary, xbool, ybool)
			r, moreErrs := convertConstToTyped(ctx, ConstBool, z, yt, untypedExpr)
			return constValue(r), append(errs, moreErrs...)
		}
	}
	return constValue{}, append(xConvErrs, ErrInvalidBinaryOperation{at(ctx, binary)})
}

func evalConstTypedBinaryExpr(ctx *Ctx, binary *BinaryExpr, xexpr, yexpr Expr) (constValue, []error) {

	// These are known not to be ConstTypes
	xt := xexpr.KnownType()[0]
	yt := yexpr.KnownType()[0]

	var zt reflect.Type
	if xt.AssignableTo(yt) {
		zt = yt
	} else if yt.AssignableTo(xt) {
		zt = xt
	} else {
		return constValue{}, []error{ErrInvalidBinaryOperation{at(ctx, binary)}}
	}

	x, xok := convertTypedToConstNumber(xexpr.Const())
	y, yok := convertTypedToConstNumber(yexpr.Const())

	if xok && yok {
		z, errs := evalConstBinaryNumericExpr(ctx, binary, x, y)
		from := reflect.Value(z).Interface().(*ConstNumber).Type
		r, moreErrs := convertConstToTyped(ctx, from, z, zt, binary)
		errs = append(errs, moreErrs...)
		return constValue(r), errs
	} else if !xok && !yok {
		switch zt.Kind() {
		case reflect.String:
			xstring := xexpr.Const().String()
			ystring := yexpr.Const().String()
			z, errs := evalConstBinaryStringExpr(ctx, binary, xstring, ystring)
			r, moreErrs := convertConstToTyped(ctx, ConstString, z, zt, binary)
			errs = append(errs, moreErrs...)
			return constValue(r), errs

		case reflect.Bool:
			xbool := xexpr.Const().Bool()
			ybool := yexpr.Const().Bool()
			z, errs := evalConstBinaryBoolExpr(ctx, binary, xbool, ybool)
			r, moreErrs := convertConstToTyped(ctx, ConstString, z, zt, binary)
			errs = append(errs, moreErrs...)
			return constValue(r), errs
		}
	} else {
		panic("go-interactive: impossible")
	}
	panic("evalConstTypedBinaryExpr unimplemented")
}
