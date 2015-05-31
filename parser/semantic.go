package parser

import (
	"fmt"
	"os"

	"github.com/ark-lang/ark-go/util"
)

// IMPORTANT NOTE for setTypeHint():
// When implementing this function for an Expr, only set the Expr's Type if
// you are on a lowest-level Expr, ie. a literal. That means, if you Expr
// contains a pointer to another Expr(s), simple pass the type hint along to that
// Expr(s) then return.

type semanticAnalyzer struct {
	file     *File
	function *Function // the function we're in, or nil if we aren't
}

func (v *semanticAnalyzer) err(err string, stuff ...interface{}) {
	/*fmt.Printf(util.TEXT_RED+util.TEXT_BOLD+"Semantic error:"+util.TEXT_RESET+" [%s:%d:%d] %s\n",
	v.peek(0).Filename, v.peek(0).LineNumber, v.peek(0).CharNumber, fmt.Sprintf(err, stuff...))*/
	fmt.Printf(util.TEXT_RED+util.TEXT_BOLD+"Semantic error:"+util.TEXT_RESET+" %s\n",
		fmt.Sprintf(err, stuff...))
	os.Exit(2)
}

func (v *semanticAnalyzer) warn(err string, stuff ...interface{}) {
	/*fmt.Printf(util.TEXT_RED+util.TEXT_BOLD+"Semantic error:"+util.TEXT_RESET+" [%s:%d:%d] %s\n",
	v.peek(0).Filename, v.peek(0).LineNumber, v.peek(0).CharNumber, fmt.Sprintf(err, stuff...))*/
	fmt.Printf(util.TEXT_RED+util.TEXT_BOLD+"Semantic warning:"+util.TEXT_RESET+" %s\n",
		fmt.Sprintf(err, stuff...))
}

func (v *semanticAnalyzer) analyze() {
	for _, node := range v.file.nodes {
		node.analyze(v)
	}
}

func (v *Block) analyze(s *semanticAnalyzer) {
	for _, n := range v.Nodes {
		n.analyze(s)
	}
}

func (v *VariableDecl) analyze(s *semanticAnalyzer) {
	v.Variable.analyze(s)
	v.Assignment.setTypeHint(v.Variable.Type)
	v.Assignment.analyze(s)
	if v.Variable.Type == nil { // type is inferred
		v.Variable.Type = v.Assignment.GetType()
	} else if v.Variable.Type != v.Assignment.GetType() {
		s.err("Cannot assign expression of type `%s` to variable of type `%s`",
			v.Assignment.GetType().TypeName(), v.Variable.Type.TypeName())
	}
}

func (v *Variable) analyze(s *semanticAnalyzer) {
	// make sure there are no illegal attributes
	s.checkDuplicateAttrs(v.Attrs)
	for _, attr := range v.Attrs {
		switch attr.Key {
		case "deprecated":
			// value is optional, nothing to check
		default:
			s.err("Invalid variable attribute key `%s`", attr.Key)
		}
	}
}

func (v *StructDecl) analyze(s *semanticAnalyzer) {
	v.Struct.analyze(s)
}

func (v *StructType) analyze(s *semanticAnalyzer) {
	// make sure there are no illegal attributes
	s.checkDuplicateAttrs(v.Attrs)
	for _, attr := range v.Attrs {
		switch attr.Key {
		case "packed":
			if attr.Value != "" {
				s.err("Struct attribute `%s` doesn't expect value", attr.Key)
			}
		case "deprecated":
			// value is optional, nothing to check
		default:
			s.err("Invalid struct attribute key `%s`", attr.Key)
		}
	}

	for _, decl := range v.Variables {
		decl.analyze(s)
	}
}

func (v *FunctionDecl) analyze(s *semanticAnalyzer) {
	v.Function.analyze(s)
}

func (v *Function) analyze(s *semanticAnalyzer) {
	// make sure there are no illegal attributes
	s.checkDuplicateAttrs(v.Attrs)
	for _, attr := range v.Attrs {
		switch attr.Key {
		case "deprecated":
			// value is optional, nothing to check
		default:
			s.err("Invalid function attribute key `%s`", attr.Key)
		}
	}

	s.function = v
	if v.Body != nil {
		v.Body.analyze(s)
	}
	s.function = nil
}

func (v *semanticAnalyzer) checkDuplicateAttrs(attrs []*Attr) {
	encountered := make(map[string]bool)
	for _, attr := range attrs {
		if encountered[attr.Key] {
			v.err("Duplicate attribute `%s`", attr.Key)
		}
		encountered[attr.Key] = true
	}
}

func (v *ReturnStat) analyze(s *semanticAnalyzer) {
	if s.function == nil {
		s.err("Return statement must be in a function")
	}

	v.Value.setTypeHint(s.function.ReturnType)
	v.Value.analyze(s)
	if v.Value.GetType() != s.function.ReturnType {
		s.err("Cannot return expression of type `%s` from function `%s` of type `%s`",
			v.Value.GetType().TypeName(), s.function.Name, s.function.ReturnType.TypeName())
	}
}
func (v *UnaryExpr) analyze(s *semanticAnalyzer) {
	v.Expr.analyze(s)

	switch v.Op {
	case UNOP_LOG_NOT:
		if v.Expr.GetType() == PRIMITIVE_bool {
			v.Type = PRIMITIVE_bool
		} else {
			s.err("Used logical not on non-bool")
		}
	case UNOP_BIT_NOT:
		if v.Expr.GetType().IsIntegerType() || v.Expr.GetType().IsFloatingType() {
			v.Type = v.Expr.GetType()
		} else {
			s.err("Used bitwise not on non-numeric type")
		}
	case UNOP_ADDRESS:
		v.Type = pointerTo(v.Expr.GetType())
		// TODO make sure v.Expr is a variable! (can't take address of a literal)
	case UNOP_DEREF:
		if ptr, ok := v.Expr.GetType().(PointerType); ok {
			v.Type = ptr.Addressee
		} else {
			s.err("Used dereference operator on non-pointer")
		}
	default:
		panic("whoops")
	}
}

func (v *UnaryExpr) setTypeHint(t Type) {
	switch v.Op {
	case UNOP_LOG_NOT:
		v.Expr.setTypeHint(PRIMITIVE_bool)
	case UNOP_BIT_NOT:
		v.Expr.setTypeHint(t)
	case UNOP_ADDRESS, UNOP_DEREF:
		v.Expr.setTypeHint(nil)
	default:
		panic("whoops")
	}
}

func (v *BinaryExpr) analyze(s *semanticAnalyzer) {
	v.Lhand.analyze(s)
	v.Rhand.analyze(s)

	switch v.Op {
	case BINOP_ADD, BINOP_SUB, BINOP_MUL, BINOP_DIV, BINOP_MOD,
		BINOP_GREATER, BINOP_LESS, BINOP_GREATER_EQ, BINOP_LESS_EQ, BINOP_EQ, BINOP_NOT_EQ,
		BINOP_BIT_AND, BINOP_BIT_OR, BINOP_BIT_XOR:
		if v.Lhand.GetType() != v.Rhand.GetType() {
			s.err("Operands for binary operator `%s` must have the same type, have `%s` and `%s`",
				v.Op.OpString(), v.Lhand.GetType().TypeName(), v.Rhand.GetType().TypeName())
		} else if lht := v.Lhand.GetType(); !(lht.IsIntegerType() || lht.IsFloatingType() || lht.LevelsOfIndirection() > 0) {
			s.err("Operands for binary operator `%s` must be numeric or pointers, have `%s`",
				v.Op.OpString(), v.Lhand.GetType().TypeName())
		} else {
			switch v.Op.Category() {
			case OP_ARITHMETIC:
				v.Type = v.Lhand.GetType()
			case OP_COMPARISON:
				v.Type = PRIMITIVE_bool
			default:
				panic("shouldn't happenen ever")
			}
		}

	case BINOP_DOT: // TODO

	case BINOP_BIT_LEFT, BINOP_BIT_RIGHT:
		if lht := v.Lhand.GetType(); !(lht.IsFloatingType() || lht.IsIntegerType() || lht.LevelsOfIndirection() > 0) {
			s.err("Left-hand operand for bitshift operator `%s` must be numeric or a pointer, have `%s`",
				v.Op.OpString(), lht.TypeName())
		} else if !v.Rhand.GetType().IsIntegerType() {
			s.err("Right-hand operatnd for bitshift operator `%s` must be an integer, have `%s`",
				v.Op.OpString(), v.Rhand.GetType().TypeName())
		} else {
			v.Type = lht
		}

	case BINOP_LOG_AND, BINOP_LOG_OR:
		if v.Lhand.GetType() != PRIMITIVE_bool || v.Rhand.GetType() != PRIMITIVE_bool {
			s.err("Operands for logical operator `%s` must have the same type, have `%s` and `%s`",
				v.Op.OpString(), v.Lhand.GetType().TypeName(), v.Rhand.GetType().TypeName())
		} else {
			v.Type = PRIMITIVE_bool
		}

	case BINOP_ASSIGN:

	default:
		panic("unimplemented bin operation")
	}
}

func (v *BinaryExpr) setTypeHint(t Type) {
	switch v.Op.Category() {
	case OP_ARITHMETIC:
		v.Lhand.setTypeHint(t)
		v.Rhand.setTypeHint(t)
	case OP_COMPARISON:
		v.Lhand.setTypeHint(nil)
		v.Rhand.setTypeHint(nil)
	case OP_BITWISE:
		v.Lhand.setTypeHint(t)
		v.Rhand.setTypeHint(t)
	case OP_LOGICAL:
		v.Lhand.setTypeHint(PRIMITIVE_bool)
		v.Rhand.setTypeHint(PRIMITIVE_bool)
	case OP_ACCESS:
		// TODO
	case OP_ASSIGN:
		// TODO
	default:
		panic("missing opcategory")
	}
}

func (v *IntegerLiteral) analyze(s *semanticAnalyzer) {}

func (v *IntegerLiteral) setTypeHint(t Type) {
	switch t {
	case PRIMITIVE_int, PRIMITIVE_uint,
		PRIMITIVE_i8, PRIMITIVE_i16, PRIMITIVE_i32, PRIMITIVE_i64, PRIMITIVE_i128,
		PRIMITIVE_u8, PRIMITIVE_u16, PRIMITIVE_u32, PRIMITIVE_u64, PRIMITIVE_u128:
		v.Type = t
	default:
		v.Type = PRIMITIVE_int // TODO check overflow
	}
}

func (v *FloatingLiteral) analyze(s *semanticAnalyzer) {}

func (v *FloatingLiteral) setTypeHint(t Type) {
	switch t {
	case PRIMITIVE_f64, PRIMITIVE_f32, PRIMITIVE_f128:
		v.Type = t
	default:
		v.Type = PRIMITIVE_f64
	}
}

func (v *StringLiteral) analyze(s *semanticAnalyzer) {}
func (v *StringLiteral) setTypeHint(t Type)          {}

func (v *RuneLiteral) analyze(s *semanticAnalyzer) {}
func (v *RuneLiteral) setTypeHint(t Type)          {}

func (v *CastExpr) analyze(s *semanticAnalyzer) {
	v.Expr.analyze(s)
	if v.Type == v.Expr.GetType() {
		s.warn("Casting expression of type `%s` to the same type",
			v.Type.TypeName())
	} else if !v.Expr.GetType().CanCastTo(v.Type) {
		s.err("Cannot cast expression of type `%s` to type `%s`",
			v.Expr.GetType().TypeName(), v.Type.TypeName())
	}
}

func (v *CastExpr) setTypeHint(t Type) {
	v.Expr.setTypeHint(nil)
}

func (v *CallExpr) analyze(s *semanticAnalyzer) {
	if len(v.Arguments) != len(v.Function.Parameters) {
		s.err("Call to `%s` expects %d arguments, have %d",
			v.Function.Name, len(v.Function.Parameters), len(v.Arguments))
	}

	for i, arg := range v.Arguments {
		arg.setTypeHint(v.Function.Parameters[i].Variable.Type)
		arg.analyze(s)
	}
}

func (v *CallExpr) setTypeHint(t Type) {}

func (v *CallStat) analyze(s *semanticAnalyzer) {
	v.Call.analyze(s)
}

func (v *CallStat) setTypeHint(t Type) {}
