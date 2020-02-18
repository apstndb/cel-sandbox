package main

import (
	"errors"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/operators"
	"github.com/k0kubun/pp"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"log"
	"time"
)

func main() {
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewIdent("request.time", decls.Timestamp, nil),
		))
	if err != nil {
		log.Fatalln(err)
	}
	parsed, issues := env.Parse(`request.time < timestamp("2020-07-01T00:00:00.000Z")`)
	if issues != nil && issues.Err() != nil {
		log.Fatalf("parse error: %s", issues.Err())
	}
	checked, issues := env.Check(parsed)
	if issues != nil && issues.Err() != nil {
		log.Fatalf("type-check error: %s", issues.Err())
	}
	result, err := evalCondition(checked.Expr(), time.Now())
	pp.Println("result:", result)
	// return
	prg, err := env.Program(checked ,cel.Globals(
		map[string]interface{}{
		// 	"request.time": ptypes.TimestampNow(),
		},
		))
	if err != nil {
		log.Fatalf("program construction error: %s", err)
	}
	pp.Println("pkg:", prg) // 'true'
	// The `out` var contains the output of a successful evaluation.
	// The `details' var would contain intermediate evaluation state if enabled as
	// a cel.ProgramOption. This can be useful for visualizing how the `out` value
	// was arrive at.
	out, details, err := prg.Eval(map[string]interface{}{
		"request.time": ptypes.TimestampNow(),
	})
	fmt.Println(out) // 'true'
	pp.Println("details:", details)
}

func evalCondition(expr *exprpb.Expr, now time.Time) (bool, error) {
	callExpr := expr.GetCallExpr()
	switch callExpr.Function {
	case operators.Less:
		return checkLessThan(callExpr.Args[0], callExpr.Args[1], now)
	case operators.Greater:
		return checkLessThan(callExpr.Args[1], callExpr.Args[0], now)
	case operators.LessEquals:
		return checkLessThanEqual(callExpr.Args[0], callExpr.Args[1], now)
	case operators.GreaterEquals:
		return checkLessThanEqual(callExpr.Args[1], callExpr.Args[0], now)
	default:
		return false, errors.New(fmt.Sprint("unknown expr:", callExpr.Function))
	}

}

const RFC3339Milli = "2006-01-02T15:04:05.999Z07:00"
func checkLessThan(lhs *exprpb.Expr, rhs *exprpb.Expr, now time.Time) (bool, error) {
	lhsTime, err := evalAsTime(lhs, now)
	if err != nil {
		return false, err
	}
	rhsTime, err := evalAsTime(rhs, now)
	if err != nil {
		return false, err
	}
	return lhsTime.Before(rhsTime), nil
}

func checkLessThanEqual(lhs *exprpb.Expr, rhs *exprpb.Expr, now time.Time) (bool, error) {
	result, err := checkLessThan(rhs, lhs, now)
	return !result, err
}

func evalAsTime(expr *exprpb.Expr, now time.Time) (time.Time, error) {
	if isRequestTime(expr) {
		return now, nil
	}
	t, err := asTime(expr)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func isRequestTime(expr *exprpb.Expr) bool {
	selectExpr := expr.GetSelectExpr()
	if selectExpr == nil {
		return false
	}
	identExpr := selectExpr.Operand.GetIdentExpr()
	if identExpr == nil {
		return false
	}
	if identExpr.Name == "request" && selectExpr.Field == "time" {
		return true
	}
	return false
}

func asTime(expr *exprpb.Expr) (time.Time, error) {
	if callExpr := expr.GetCallExpr(); callExpr != nil && callExpr.Function != "timestamp" {
		return time.Time{}, errors.New("expr is not timestamp")
	}
	str := expr.GetCallExpr().Args[0].GetConstExpr().GetStringValue()
	return time.Parse(RFC3339Milli, str)
}