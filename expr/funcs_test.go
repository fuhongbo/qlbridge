package expr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fuhongbo/qlbridge/expr"
	"github.com/fuhongbo/qlbridge/expr/builtins"
)

func TestFuncsRegistry(t *testing.T) {
	t.Parallel()

	builtins.LoadAllBuiltins()
	_, ok := expr.EmptyEvalFunc(nil, nil)
	assert.Equal(t, false, ok)

}
