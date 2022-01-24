package plan_test

import (
	"os"
	"testing"

	td "github.com/fuhongbo/qlbridge/datasource/mockcsvtestdata"
	"github.com/fuhongbo/qlbridge/testutil"
)

func TestMain(m *testing.M) {
	testutil.Setup() // will call flag.Parse()

	// load our mock data sources "users", "articles"
	td.LoadTestDataOnce()

	// Now run the actual Tests
	os.Exit(m.Run())
}

func TestRunTestSuite(t *testing.T) {
	testutil.RunDDLTests(t)
	testutil.RunTestSuite(t)
}
