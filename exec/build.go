package exec

import (
	u "github.com/araddon/gou"

	"github.com/araddon/qlbridge/expr"
	"github.com/araddon/qlbridge/plan"
)

var (
	_ = u.EMPTY

	// Ensure that we implement the expr.Visitor interface
	_ expr.Visitor       = (*JobBuilder)(nil)
	_ plan.SourceVisitor = (*JobBuilder)(nil)
)

// This is a simple, single source Job Executor
//   hopefully we create smarter ones but this is a basic implementation for
///  running in-process, not distributed
type JobBuilder struct {
	expr.Visitor
	Ctx      *plan.Context
	where    expr.Node
	distinct bool
	children Tasks
}

// JobBuilder
//   @conf   = the config/runtime schema info
//   @connInfo = connection string info for original connection
//
func NewJobBuilder(reqCtx *plan.Context) *JobBuilder {
	b := JobBuilder{}
	b.Ctx = reqCtx
	return &b
}

func (m *JobBuilder) Wrap(visitor expr.Visitor) expr.Visitor {
	u.Debugf("wrap %T", visitor)
	m.Visitor = visitor
	return m
}
func (m *JobBuilder) VisitPreparedStmt(stmt *expr.PreparedStatement) (expr.Task, expr.VisitStatus, error) {
	u.Debugf("VisitPreparedStmt %+v", stmt)
	return nil, expr.VisitFinal, expr.ErrNotImplemented
}

func (m *JobBuilder) VisitCommand(stmt *expr.SqlCommand) (expr.Task, expr.VisitStatus, error) {
	u.Debugf("VisitCommand %+v", stmt)
	return nil, expr.VisitFinal, expr.ErrNotImplemented
}