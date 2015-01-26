package expr

import (
	"fmt"
	"runtime"
	"strings"

	u "github.com/araddon/gou"
	"github.com/araddon/qlbridge/lex"
)

var _ = u.EMPTY

// We have a default Dialect, which is the "Language" or rule-set of ql
var DefaultDialect *lex.Dialect = lex.LogicalExpressionDialect

// TokenPager wraps a Lexer, and implements the Logic to determine what is
// the end of this particular clause
//
//    SELECT * FROM X   --   keyword FROM identifies end of columns
//    SELECT x, y, cast(item,string) AS item_str FROM product  -- commas, FROM are end of columns
//
type TokenPager interface {
	Peek() lex.Token
	Next() lex.Token
	Cur() lex.Token
	Last() lex.TokenType
	Backup()
	IsEnd() bool
}

// SchemaInfo
//
type SchemaInfo interface {
	Key() string
}

// TokenPager is responsible for determining end of
// current tree (column, etc)
type LexTokenPager struct {
	done   bool
	tokens []lex.Token // list of all the tokens
	cursor int
	lex    *lex.Lexer
	end    lex.TokenType
}

func NewLexTokenPager(lex *lex.Lexer) *LexTokenPager {
	p := LexTokenPager{
		lex: lex,
	}
	p.cursor = -1
	return &p
}

// next returns the next token.
func (m *LexTokenPager) Next() lex.Token {
	if !m.done {
		tok := m.lex.NextToken()
		if tok.T == lex.TokenEOF {
			m.done = true
		}
		m.tokens = append(m.tokens, tok)
		//u.Infof("next: %v of %v cur=%v", m.cursor, len(m.tokens), tok)
	}
	if m.cursor+1 < len(m.tokens) {
		m.cursor++
		//u.Infof("increment cursor: %v of %v %v", m.cursor, len(m.tokens), m.cursor < len(m.tokens))
	}
	return m.tokens[m.cursor]
}
func (m *LexTokenPager) Cur() lex.Token {
	if m.cursor == -1 {
		return m.Next()
	}
	return m.tokens[m.cursor]
}
func (m *LexTokenPager) Last() lex.TokenType {
	return m.end
}
func (m *LexTokenPager) IsEnd() bool {
	return false
}

// backup backs the input stream up one token.
func (m *LexTokenPager) Backup() {
	if m.cursor > 0 {
		m.cursor--
		//u.Warnf("Backup?: %v", m.cursor)
		return
	}
}

// peek returns but does not consume the next token.
func (m *LexTokenPager) Peek() lex.Token {
	//u.Infof("prepeek: %v of %v", m.cursor, len(m.tokens))
	if len(m.tokens) <= m.cursor+1 {
		m.Next()
		m.cursor--
		//u.Warnf("decrement cursor?: %v %p", m.cursor, &m.cursor)
	}
	//u.Infof("peek:  %v of %v %v", m.cursor, len(m.tokens), m.tokens[m.cursor+1])
	return m.tokens[m.cursor+1]
}

// Tree is the representation of a single parsed expression
type Tree struct {
	runCheck   bool
	Root       Node // top-level root node of the tree
	TokenPager      // pager for grabbing next tokens, backup(), recognizing end
}

func NewTree(pager TokenPager) *Tree {
	t := Tree{TokenPager: pager}
	return &t
}

// Parse a single Expression, returning a Tree
//
//    ParseExpression("5 * toint(item_name)")
//
func ParseExpression(expressionText string) (*Tree, error) {
	l := lex.NewLexer(expressionText, lex.LogicalExpressionDialect)
	pager := NewLexTokenPager(l)
	t := NewTree(pager)
	pager.end = lex.TokenEOF
	err := t.BuildTree(true)
	return t, err
}

// Parsing.

// errorf formats the error and terminates processing.
func (t *Tree) errorf(format string, args ...interface{}) {
	t.Root = nil
	format = fmt.Sprintf("expr: %s", format)
	msg := fmt.Errorf(format, args...)
	u.LogTracef(u.WARN, "about to panic: %v", msg)
	panic(msg)
}

// error terminates processing.
func (t *Tree) error(err error) {
	t.errorf("%s", err)
}

// expect verifies the current token and guarantees it has the required type
func (t *Tree) expect(expected lex.TokenType, context string) lex.Token {
	token := t.Cur()
	//u.Debugf("checking expected? %v got?: %v", expected, token)
	if token.T != expected {
		u.Warnf("unexpeted token? %v want:%v", token, expected)
		t.unexpected(token, context)
	}
	return token
}

// expectOneOf consumes the next token and guarantees it has one of the required types.
func (t *Tree) expectOneOf(expected1, expected2 lex.TokenType, context string) lex.Token {
	token := t.Cur()
	if token.T != expected1 && token.T != expected2 {
		t.unexpected(token, context)
	}
	return token
}

// unexpected complains about the token and terminates processing.
func (t *Tree) unexpected(token lex.Token, context string) {
	u.Errorf("unexpected?  %v", token)
	t.errorf("unexpected %s in %s", token, context)
}

// recover is the handler that turns panics into returns from the top level of Parse.
func (t *Tree) recover(errp *error) {
	e := recover()
	if e != nil {
		u.Errorf("Recover():  %v", e)
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		*errp = e.(error)
	}
	return
}

// buildTree take the tokens and recursively build into expression tree node
// @runCheck  Do we want to verify this tree?   If being used as VM then yes.
func (t *Tree) BuildTree(runCheck bool) error {
	//u.Debugf("parsing: %v", t.Cur())
	t.runCheck = runCheck
	//u.Debugf("parsing: %v", t.Cur())
	t.Root = t.O(0)
	//u.Debugf("after parse()")
	if !t.IsEnd() {
		//u.Warnf("Not End? last=%v", t.TokenPager.Last())
		//t.expect(t.TokenPager.Last(), "input")
	}
	if runCheck {
		if err := t.Root.Check(); err != nil {
			u.Errorf("found error: %v", err)
			t.error(err)
			return err
		}
	}

	return nil
}

/*

Operator Predence planner during parse phase:
  when we parse and build our node-sub-node structures we need to plan
  the precedence rules, we use a recursion tree to build this

http://dev.mysql.com/doc/refman/5.0/en/operator-precedence.html
https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Operator_Precedence
http://www.postgresql.org/docs/9.4/static/sql-syntax-lexical.html#SQL-PRECEDENCE

TODO:
 - implement new one for parens
 - implement flags for commutative/
--------------------------------------
O -> A {( "||" | OR  ) A}
A -> C {( "&&" | AND ) C}
C -> P {( "==" | "!=" | ">" | ">=" | "<" | "<=" | "LIKE" | "IN" ) P}
P -> M {( "+" | "-" ) M}
M -> F {( "*" | "/" ) F}
F -> v | "(" O ")" | "!" O | "-" O
v -> number | func(..)
Func -> name "(" param {"," param} ")"
param -> number | "string" | O



Recursion:  We recurse so the LAST to evaluate is the highest (parent, then or)
   ie the deepest we get in recursion tree is the first to be evaluated

1	Unary + - arithmetic operators, PRIOR operator
2	* / arithmetic operators
3	Binary + - arithmetic operators, || character operators
4	All comparison operators
5	NOT logical operator
6	AND logical operator
7	OR logical operator
8   Paren's


*/

// expr:
func (t *Tree) O(depth int) Node {
	//u.Debugf("%d t.O Cur(): %v", depth, t.Cur())
	n := t.A(depth)
	//u.Debugf("%d t.O AFTER: n:%v cur:%v %v", depth, n, t.Cur(), t.Peek())
	for {
		tok := t.Cur()
		//u.Debugf("tok:  cur=%v peek=%v", t.Cur(), t.Peek())
		switch tok.T {
		case lex.TokenLogicOr, lex.TokenOr:
			t.Next()
			n = NewBinaryNode(tok, n, t.A(depth+1))
		case lex.TokenCommentSingleLine:
			// we consume the comment signifier "--""   as well as comment
			//u.Debugf("tok:  %v", t.Next())
			//u.Debugf("tok:  %v", t.Next())
			t.Next()
			t.Next()
		case lex.TokenEOF, lex.TokenEOS, lex.TokenFrom, lex.TokenComma, lex.TokenIf,
			lex.TokenAs, lex.TokenSelect, lex.TokenLimit:
			// these are indicators of End of Current Clause, so we can return?
			//u.Debugf("done, return: %v", tok)
			return n
		default:
			//u.Debugf("root couldnt evaluate node? %v", tok)
			return n
		}
	}
}

func (t *Tree) A(depth int) Node {
	//u.Debugf("%d t.A: %v", depth, t.Cur())
	n := t.C(depth)
	//u.Debugf("%d t.A: AFTER %v", depth, t.Cur())
	for {
		//u.Debugf("tok:  cur=%v peek=%v", t.Cur(), t.Peek())
		switch tok := t.Cur(); tok.T {
		case lex.TokenLogicAnd, lex.TokenAnd:
			t.Next()
			n = NewBinaryNode(tok, n, t.C(depth+1))
		default:
			return n
		}
	}
}

func (t *Tree) C(depth int) Node {
	//u.Debugf("%d t.C: %v", depth, t.Cur())
	n := t.P(depth)
	//u.Debugf("%d t.C: %v", depth, t.Cur())
	for {
		//u.Debugf("tok:  cur=%v peek=%v", t.Cur(), t.Peek())
		switch cur := t.Cur(); cur.T {
		case lex.TokenEqual, lex.TokenEqualEqual, lex.TokenNE, lex.TokenGT, lex.TokenGE,
			lex.TokenLE, lex.TokenLT, lex.TokenLike:
			t.Next()
			n = NewBinaryNode(cur, n, t.P(depth+1))
		case lex.TokenBetween:
			// weird syntax:    BETWEEN x AND y     AND is ignored essentially
			t.Next()
			n2 := t.P(depth)
			t.expect(lex.TokenLogicAnd, "input")
			t.Next()
			u.Infof("Between: %v %v", t.Cur(), t.Peek())
			n = NewTriNode(cur, n, n2, t.P(depth+1))
		case lex.TokenIN:
			t.Next()
			// This isn't really a Binary?   It is an array or
			// other type of native data type?
			//n = NewSet(cur, n, t.Set(depth+1))
			return t.MultiArg(n, cur, depth)
		default:
			return n
		}
	}
}

func (t *Tree) P(depth int) Node {
	//u.Debugf("%d t.P: %v", depth, t.Cur())
	n := t.M(depth)
	//u.Debugf("%d t.P: AFTER %v", depth, t.Cur())
	for {
		switch cur := t.Cur(); cur.T {
		case lex.TokenPlus, lex.TokenMinus:
			t.Next()
			n = NewBinaryNode(cur, n, t.M(depth+1))
		default:
			return n
		}
	}
}

func (t *Tree) M(depth int) Node {
	//u.Debugf("%d t.M: %v", depth, t.Cur())
	n := t.F(depth)
	//u.Debugf("%d t.M after: %v  %v", depth, t.Cur(), n)
	for {
		switch cur := t.Cur(); cur.T {
		case lex.TokenStar, lex.TokenMultiply, lex.TokenDivide, lex.TokenModulus:
			t.Next()
			n = NewBinaryNode(cur, n, t.F(depth+1))
		default:
			return n
		}
	}
}

func (t *Tree) MultiArg(first Node, op lex.Token, depth int) Node {
	//u.Debugf("%d t.MultiArg: %v", depth, t.Cur())
	t.expect(lex.TokenLeftParenthesis, "input")
	t.Next() // Consume Left Paren
	//u.Debugf("%d t.MultiArg after: %v ", depth, t.Cur())
	multiNode := NewMultiArgNode(op)
	multiNode.Append(first)
	for {
		//u.Debugf("MultiArg iteration: %v", t.Cur())
		switch cur := t.Cur(); cur.T {
		case lex.TokenRightParenthesis:
			t.Next() // Consume the Paren
			return multiNode
		case lex.TokenComma:
			t.Next()
		default:
			n := t.O(depth)
			if n != nil {
				multiNode.Append(n)
			} else {
				u.Warnf("invalid?  %v", t.Cur())
				return multiNode
			}
		}
	}
}

func (t *Tree) F(depth int) Node {
	//u.Debugf("%d t.F: %v", depth, t.Cur())
	switch cur := t.Cur(); cur.T {
	case lex.TokenUdfExpr:
		return t.v(depth)
	case lex.TokenInteger, lex.TokenFloat:
		return t.v(depth)
	case lex.TokenIdentity:
		return t.v(depth)
	case lex.TokenValue:
		return t.v(depth)
	case lex.TokenStar:
		// in special situations:   count(*) ??
		return t.v(depth)
	case lex.TokenNegate, lex.TokenMinus:
		t.Next()
		return NewUnary(cur, t.F(depth+1))
	case lex.TokenLeftParenthesis:
		// I don't think this is right, parens should be higher up
		// in precedence stack, very top?
		t.Next() // Consume the Paren
		n := t.O(depth + 1)
		if bn, ok := n.(*BinaryNode); ok {
			bn.Paren = true
		}
		//u.Debugf("expects right paren? cur=%v p=%v", t.Cur(), t.Peek())
		t.expect(lex.TokenRightParenthesis, "input")
		t.Next()
		return n
	default:
		u.Warnf("unexpected? %v", cur)
		//t.unexpected(cur, "input")
		panic(fmt.Sprintf("unexpected token %v ", cur))
	}
	return nil
}

func (t *Tree) v(depth int) Node {
	//u.Debugf("%d t.v: cur(): %v   peek:%v", depth, t.Cur(), t.Peek())
	switch cur := t.Cur(); cur.T {
	case lex.TokenInteger, lex.TokenFloat:
		n, err := NewNumber(Pos(cur.Pos), cur.V)
		if err != nil {
			t.error(err)
		}
		t.Next()
		return n
	case lex.TokenValue:
		n := NewStringNode(Pos(cur.Pos), cur.V)
		t.Next()
		return n
	case lex.TokenIdentity:
		n := NewIdentityNode(Pos(cur.Pos), cur.V)
		t.Next()
		return n
	case lex.TokenStar:
		n := NewStringNode(Pos(cur.Pos), cur.V)
		t.Next()
		return n
	case lex.TokenUdfExpr:
		//u.Debugf("%v t.v calling Func()?: %v", depth, cur)
		return t.Func(depth, cur)
	case lex.TokenLeftParenthesis:
		// I don't think this is right, it should be higher up
		// in precedence stack, very top?
		t.Next()
		n := t.O(depth + 1)
		if bn, ok := n.(*BinaryNode); ok {
			bn.Paren = true
		}
		//u.Debugf("cur?%v n %v  ", t.Cur(), n.StringAST())
		t.Next()
		t.expect(lex.TokenRightParenthesis, "input")
		return n
	default:
		if t.IsEnd() {
			return nil
		}
		//u.Warnf("Unexpected?: %v", cur)
		t.unexpected(cur, "input")
	}
	t.Backup()
	return nil
}

func (t *Tree) Func(depth int, tok lex.Token) (fn *FuncNode) {
	//u.Debugf("%v Func tok: %v cur:%v peek:%v", depth, tok.V, t.Cur().V, t.Peek().V)
	token := tok
	if t.Peek().T != lex.TokenLeftParenthesis {
		panic("must have left paren on function")
	}
	// if t.Peek().T == lex.TokenLeftParenthesis {
	// 	token = tok
	// } else {
	// 	//token = t.Next()
	// }

	var node Node
	//var err error

	funcImpl, ok := t.getFunction(token.V)
	if !ok {
		if t.runCheck {
			//u.Warnf("non func? %v", token.V)
			t.errorf("non existent function %s", token.V)
		} else {
			// if we aren't testing for validity, make a "fake" func
			// we may not be using vm, just ast
			//u.Warnf("non func? %v", token.V)
			funcImpl = Func{Name: token.V}
		}
	}
	fn = NewFuncNode(Pos(token.Pos), token.V, funcImpl)
	//u.Debugf("%d t.Func()?: %v %v", depth, t.Cur(), t.Peek())
	t.Next() // step forward to hopefully left paren
	t.expect(lex.TokenLeftParenthesis, "func")

	for {
		node = nil
		t.Next() // Are we sure we consume?
		//u.Infof("%d pre loop token?: cur=%v peek=%v", depth, t.Cur(), t.Peek())
		switch firstToken := t.Cur(); firstToken.T {
		case lex.TokenRightParenthesis:
			t.Next()
			if node != nil {
				fn.append(node)
			}
			//u.Warnf(" right paren? ")
			return
		case lex.TokenEOF, lex.TokenEOS, lex.TokenFrom:
			//u.Warnf("return: %v", t.Cur())
			if node != nil {
				fn.append(node)
			}
			return
		default:
			//u.Debugf("%v getting node? t.Func()?: %v", depth, firstToken)
			node = t.O(depth + 1)
		}

		token = t.Cur()
		//u.Infof("%d Func() pt2 consumed token?: %v", depth, token)
		switch token.T {
		case lex.TokenComma:
			if node != nil {
				fn.append(node)
			}
			// continue
		case lex.TokenRightParenthesis:
			if node != nil {
				fn.append(node)
			}
			t.Next()
			//u.Warnf("found right paren %v", t.Cur())
			return
		case lex.TokenEOF, lex.TokenEOS, lex.TokenFrom:
			if node != nil {
				fn.append(node)
			}
			t.Next()
			//u.Debugf("return: %v", t.Cur())
			return
		case lex.TokenEqual, lex.TokenEqualEqual, lex.TokenNE, lex.TokenGT, lex.TokenGE,
			lex.TokenLE, lex.TokenLT, lex.TokenStar, lex.TokenMultiply, lex.TokenDivide:
			// this func arg is an expression
			//     toint(str_item * 5)

			//t.Backup()
			//u.Debugf("hmmmmm:  %v  cu=%v", token, t.Cur())
			node = t.O(depth + 1)
			if node != nil {
				fn.append(node)
			}
		default:
			t.unexpected(token, "func")
		}
	}
}

// get Function from Global
func (t *Tree) getFunction(name string) (v Func, ok bool) {
	if v, ok = funcs[strings.ToLower(name)]; ok {
		return
	}
	return
}

func (t *Tree) String() string {
	return t.Root.String()
}