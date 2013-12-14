package interactive

import "reflect"

// Constant types, should not be used with the reflect package, but
// can annotate information needed for evaluating const expressions
type ConstType interface {
	reflect.Type
	IsIntegral() bool
	IsReal() bool
}

type ConstIntType struct { reflect.Type }
type ConstRuneType struct { reflect.Type }
type ConstFloatType struct { reflect.Type }
type ConstComplexType struct { reflect.Type }
type ConstStringType struct { reflect.Type }
type ConstNilType struct { reflect.Type }
type ConstBoolType struct { reflect.Type }

var (
	ConstInt = ConstIntType { reflect.TypeOf(0) }
	ConstRune = ConstRuneType { reflect.TypeOf('\000') }
	ConstFloat = ConstFloatType { reflect.TypeOf(0.0) }
	ConstComplex = ConstComplexType { reflect.TypeOf(0i) }
	ConstString = ConstStringType { reflect.TypeOf("") }
	ConstNil = ConstNilType { nil }
	ConstBool = ConstBoolType { reflect.TypeOf(false) }
)

func (ConstIntType) String() string { return "int" }
func (ConstRuneType) String() string { return "rune" }
func (ConstFloatType) String() string { return "float64" }
func (ConstComplexType) String() string { return "complex128" }
func (ConstStringType) String() string { return "string" }
func (ConstNilType) String() string { return "<T>" }
func (ConstBoolType) String() string { return "bool" }

func (ConstIntType) IsIntegral() bool { return true }
func (ConstRuneType) IsIntegral() bool { return true }
func (ConstFloatType) IsIntegral() bool { return false }
func (ConstComplexType) IsIntegral() bool { return false }
func (ConstStringType) IsIntegral() bool { return false }
func (ConstNilType) IsIntegral() bool { return false }
func (ConstBoolType) IsIntegral() bool { return false }

func (ConstIntType) IsReal() bool { return true }
func (ConstRuneType) IsReal() bool { return true }
func (ConstFloatType) IsReal() bool { return true }
func (ConstComplexType) IsReal() bool { return false }
func (ConstStringType) IsReal() bool { return false }
func (ConstNilType) IsReal() bool { return false }
func (ConstBoolType) IsReal() bool { return false }

// Returns the ConstType of a binary, non-boolean, expression invalving const types of
// x and y.
// Errors match those produced by gc and are as follows:
// If one type is a numeric type, and the other is not
//	ErrBadConversion other -> numeric
// If string and bool
//	ErrBadConversion bool -> int
//	ErrBadConversion string -> int
// If nil and string
//	ErrBadConversion nil -> int
//	ErrBadConversion string -> int
// If nil and bool
//	ErrBadConversion nil -> bool
//
func promoteConsts(ctx *Ctx, x, y ConstType, xexpr, yexpr Expr, xval, yval reflect.Value) (ConstType, []error) {
	switch x.(type) {
	case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
		switch y.(type) {
		case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
			return promoteConstNumbers(x, y), nil
		}
		return nil, []error{ErrBadConversion{at(ctx, yexpr), y, x, yval}}
	case ConstStringType:
		switch y.(type) {
		case ConstStringType:
			return x, nil
		case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
			return nil, []error{ErrBadConversion{at(ctx, yexpr), x, y, xval}}
		default:
			return nil, []error{
				ErrBadConversion{at(ctx, yexpr), x, ConstInt, xval},
				ErrBadConversion{at(ctx, yexpr), y, ConstInt, yval},
			}
		}
	case ConstNilType:
		print("yay\n")
		switch y.(type) {
		case ConstNilType:
			print("nil\n")
			return x, nil
		case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
			print("num\n")
			return nil, []error{ErrBadConversion{at(ctx, yexpr), x, y, xval}}
		case ConstBoolType:
			print("bool\n")
			return nil, []error{ErrBadConversion{at(ctx, yexpr), x, ConstInt, xval}}
		default:
			print("other\n")
			return nil, []error{
				ErrBadConversion{at(ctx, yexpr), x, ConstInt, xval},
				ErrBadConversion{at(ctx, yexpr), y, ConstInt, yval},
			}
		}
	case ConstBoolType:
		switch y.(type) {
		case ConstBoolType:
			return x, nil
		case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
			return nil, []error{ErrBadConversion{at(ctx, yexpr), x, y, xval}}
		case ConstNilType:
			return nil, []error{ErrBadConversion{at(ctx, yexpr), y, x, yval}}
		default:
			return nil, []error{
				ErrBadConversion{at(ctx, yexpr), x, ConstInt, xval},
				ErrBadConversion{at(ctx, yexpr), y, ConstInt, yval},
			}
		}
	}
	panic("go-interactive: impossible")
}

// Can't fail, but panics if x or y are not Const(Int|Rune|Float|Complex)Type
func promoteConstNumbers(x, y ConstType) ConstType {
	switch x.(type) {
	case ConstIntType:
		switch y.(type) {
		case ConstIntType:
			return x
		case ConstRuneType, ConstFloatType, ConstComplexType:
			return y
		}
	case ConstRuneType:
		switch y.(type) {
		case ConstIntType, ConstRuneType:
			return x
		case ConstFloatType, ConstComplexType:
			return y
		}
	case ConstFloatType:
		switch y.(type) {
		case ConstIntType, ConstRuneType, ConstFloatType:
			return x
		case ConstComplexType:
			return y
		}
	case ConstComplexType:
		switch y.(type) {
		case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
			return x
		}
	}
	panic("go-interactive: promoteConstNumbers called with non-numbers")
}

func convertConstToTyped(ctx *Ctx, from ConstType, c constValue, to reflect.Type, expr Expr) (
	v reflect.Value, errs []error) {

	v = reflect.New(to).Elem()

	switch from.(type) {
	case ConstIntType, ConstRuneType, ConstFloatType, ConstComplexType:
		underlying := reflect.Value(c).Interface().(*ConstNumber)
		switch to.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			var errs []error
			i, truncation, overflow := underlying.Value.Int(to.Bits())
			if truncation {
				errs = append(errs, ErrTruncatedConstant{at(ctx, expr), ConstInt, underlying})
			}
			if overflow {
				integer, _ := underlying.Value.Integer()
				errs = append(errs, ErrOverflowedConstant{at(ctx, expr), from, to, integer})
			}
			v.SetInt(i)
			return v, errs

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u, truncation, overflow := underlying.Value.Uint(to.Bits())
			if truncation {
				errs = append(errs, ErrTruncatedConstant{at(ctx, expr), ConstInt, underlying})
			}
			if overflow {
				integer, _ := underlying.Value.Integer()
				errs = append(errs, ErrOverflowedConstant{at(ctx, expr), from, to, integer})
			}
			v.SetUint(u)
			return v, errs

		case reflect.Float32, reflect.Float64:
			f, truncation, _ := underlying.Value.Float64()
			if truncation {
				errs = append(errs, ErrTruncatedConstant{at(ctx, expr), ConstFloat, underlying})
			}
			v.SetFloat(f)
			return v, errs

		case reflect.Complex64, reflect.Complex128:
			cmplx, _ := underlying.Value.Complex128()
			v.SetComplex(cmplx)
			return v, errs
		}
	case ConstStringType:
		// Check Kind == String ourselves, as reflect.Value.String() doesn't panic
		// on non string values
		if v.Type().Kind() != reflect.String {
			panic("go-interactive: string constant has wrong underlying type")
		}
		v.SetString(reflect.Value(c).String())

	case ConstBoolType:
		v.SetBool(reflect.Value(c).Bool())

	case ConstNilType:
		// Unfortunately there is no reflect.Type.CanNil()
		switch to.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface,
			reflect.Map, reflect.Ptr, reflect.Slice:

			// v is already nil
			return v, nil
		}
	}
	return v, []error{ErrBadConversion{at(ctx, expr), from, to, reflect.Value(c)}}
}
