// Copyright 2015 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
//
// Author: Tamir Duberstein (tamird@gmail.com)

package parser

import (
	"go/constant"
	"go/token"
	"reflect"
	"regexp"
	"testing"

	"github.com/cockroachdb/cockroach/testutils"
)

func TestTypeCheck(t *testing.T) {
	testData := []string{
		`NULL + 1`,
		`NULL + 1.1`,
		`NULL + '2006-09-23'::date`,
		`NULL + '1h'::interval`,
		`NULL + 'hello'`,
		`NULL::int`,
		`NULL + 'hello'::bytes`,
		`(1.1::decimal)::decimal`,
		`NULL = 1`,
		`1 = NULL`,
		`true AND NULL`,
		`NULL OR false`,
		`1 IN (SELECT 1)`,
		`IF(true, 2, 3)`,
		`IF(false, 2, 3)`,
		`IF(NULL, 2, 3)`,
		`IF(NULL, 2, 3.0)`,
		`IF(true, (1, 2), (1, 3))`,
		`IFNULL(1, 2)`,
		`IFNULL(1, 2.0)`,
		`IFNULL(NULL, 2)`,
		`IFNULL(2, NULL)`,
		`IFNULL((1, 2), (1, 3))`,
		`NULLIF(1, 2)`,
		`NULLIF(1, 2.0)`,
		`NULLIF(NULL, 2)`,
		`NULLIF(2, NULL)`,
		`NULLIF((1, 2), (1, 3))`,
		`COALESCE(1, 2, 3, 4, 5)`,
		`COALESCE(1, 2.0)`,
		`COALESCE(NULL, 2)`,
		`COALESCE(2, NULL)`,
		`COALESCE((1, 2), (1, 3))`,
		`true IS NULL`,
		`true IS NOT NULL`,
		`true IS TRUE`,
		`true IS NOT TRUE`,
		`true IS FALSE`,
		`true IS NOT FALSE`,
		`CASE 1 WHEN 1 THEN (1, 2) ELSE (1, 3) END`,
		`1 BETWEEN 2 AND 3`,
		`COUNT(3)`,
	}
	for _, d := range testData {
		expr, err := ParseExprTraditional(d)
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if _, err := TypeCheck(expr, nil, NoTypePreference); err != nil {
			t.Errorf("%s: unexpected error %s", d, err)
		}
	}
}

func TestTypeCheckError(t *testing.T) {
	testData := []struct {
		expr     string
		expected string
	}{
		{`'1' + '2'`, `unsupported binary operator:`},
		{`'a' + 0`, `unsupported binary operator:`},
		{`1.1 # 3.1`, `unsupported binary operator:`},
		{`~0.1`, `unsupported unary operator:`},
		{`'10' > 2`, `unsupported comparison operator:`},
		{`a`, `qualified name "a" not found`},
		{`1 AND true`, `incompatible AND argument type: int`},
		{`1.0 AND true`, `incompatible AND argument type: float`},
		{`'a' OR true`, `incompatible OR argument type: string`},
		{`(1, 2) OR true`, `incompatible OR argument type: tuple`},
		{`NOT 1`, `incompatible NOT argument type: int`},
		{`lower()`, `unknown signature for lower: lower()`},
		{`lower(1, 2)`, `unknown signature for lower: lower(int, int)`},
		{`lower(1)`, `unknown signature for lower: lower(int)`},
		{`1::date`, `invalid cast: int -> DATE`},
		{`1::timestamp`, `invalid cast: int -> TIMESTAMP`},
		{`CASE 'one' WHEN 1 THEN 1 WHEN 'two' THEN 2 END`, `incompatible condition type`},
		{`CASE 1 WHEN 1 THEN 'one' WHEN 2 THEN 2 END`, `incompatible value type`},
		{`CASE 1 WHEN 1 THEN 'one' ELSE 2 END`, `incompatible value type`},
		{`(1, 2, 3) = (1, 2)`, `unequal number of entries in tuple expressions`},
		{`(1, 2) = (1, 'a')`, `unsupported comparison operator`},
		{`1 IN ('a', 'b')`, `unsupported comparison operator:`},
		{`1 IN (1, 'a')`, `unsupported comparison operator`},
		{`1.0 BETWEEN 2 AND '5'`, `expected 1.0 to be of type string, found type float`},
		{`IF(1, 2, 3)`, `incompatible IF condition type: int`},
		{`IF(true, 2, '5')`, `incompatible IF expressions: expected 2 to be of type string, found type int`},
		{`IFNULL(1, '5')`, `incompatible IFNULL expressions: expected 1 to be of type string, found type int`},
		{`NULLIF(1, '5')`, `incompatible NULLIF expressions: expected 1 to be of type string, found type int`},
		{`COALESCE(1, 2, 3, 4, '5')`, `incompatible COALESCE expressions: expected 1 to be of type string, found type int`},
	}
	for _, d := range testData {
		expr, err := ParseExprTraditional(d.expr)
		if err != nil {
			t.Fatalf("%s: %v", d.expr, err)
		}
		if _, err := TypeCheck(expr, nil, NoTypePreference); !testutils.IsError(err, regexp.QuoteMeta(d.expected)) {
			t.Errorf("%s: expected %s, but found %v", d.expr, d.expected, err)
		}
	}
}

func intConst(s string) Expr {
	return &NumVal{Value: constant.MakeFromLiteral(s, token.INT, 0), OrigString: s}
}

func floatConst(s string) Expr {
	return &NumVal{Value: constant.MakeFromLiteral(s, token.FLOAT, 0), OrigString: s}
}

func cloneMapArgs(args MapArgs) MapArgs {
	clone := make(MapArgs)
	for k, v := range args {
		clone[k] = v
	}
	return clone
}

func forEachPerm(exprs []Expr, i int, fn func([]Expr)) {
	if i == len(exprs)-1 {
		fn(exprs)
	}
	for j := i; j < len(exprs); j++ {
		exprs[i], exprs[j] = exprs[j], exprs[i]
		forEachPerm(exprs, i+1, fn)
		exprs[i], exprs[j] = exprs[j], exprs[i]
	}
}

func TestTypeCheckSameTypedExprs(t *testing.T) {
	mapArgsInt := MapArgs{"a": TypeInt}
	mapArgsFloat := MapArgs{"a": TypeFloat}
	mapArgsIntAndFloat := MapArgs{"a": TypeFloat, "b": TypeFloat}

	testData := []struct {
		args    MapArgs
		desired Datum
		exprs   []Expr

		expectedType Datum
		expectedArgs MapArgs
	}{
		// Constants.
		{nil, nil, []Expr{intConst("1")}, TypeInt, nil},
		{nil, nil, []Expr{floatConst("1")}, TypeFloat, nil},
		{nil, nil, []Expr{intConst("1"), floatConst("1")}, TypeFloat, nil},
		// Resolved exprs.
		{nil, nil, []Expr{NewDInt(1)}, TypeInt, nil},
		{nil, nil, []Expr{NewDFloat(1)}, TypeFloat, nil},
		// Mixing constants and resolved exprs.
		{nil, nil, []Expr{NewDInt(1), intConst("1")}, TypeInt, nil},
		{nil, nil, []Expr{NewDInt(1), floatConst("1")}, TypeInt, nil}, // This is what the AST would look like after folding (0.6 + 0.4).
		{nil, nil, []Expr{NewDInt(1), NewDInt(1)}, TypeInt, nil},
		{nil, nil, []Expr{NewDFloat(1), intConst("1")}, TypeFloat, nil},
		{nil, nil, []Expr{NewDFloat(1), floatConst("1")}, TypeFloat, nil},
		{nil, nil, []Expr{NewDFloat(1), NewDFloat(1)}, TypeFloat, nil},
		// Mixing resolved constants and resolved exprs with MapArgs.
		{mapArgsFloat, nil, []Expr{NewDFloat(1), ValArg{"a"}}, TypeFloat, mapArgsFloat},
		{mapArgsFloat, nil, []Expr{intConst("1"), ValArg{"a"}}, TypeFloat, mapArgsFloat},
		{mapArgsFloat, nil, []Expr{floatConst("1"), ValArg{"a"}}, TypeFloat, mapArgsFloat},
		{mapArgsInt, nil, []Expr{intConst("1"), ValArg{"a"}}, TypeInt, mapArgsInt},
		{mapArgsInt, nil, []Expr{floatConst("1"), ValArg{"a"}}, TypeInt, mapArgsInt},
		{mapArgsIntAndFloat, nil, []Expr{ValArg{"b"}, ValArg{"a"}}, TypeFloat, mapArgsIntAndFloat},
		// Mixing unresolved constants and resolved exprs with MapArgs.
		{nil, nil, []Expr{NewDFloat(1), ValArg{"a"}}, TypeFloat, mapArgsFloat},
		{nil, nil, []Expr{intConst("1"), ValArg{"a"}}, TypeInt, mapArgsInt},
		{nil, nil, []Expr{floatConst("1"), ValArg{"a"}}, TypeFloat, mapArgsFloat},
		// Verify dealing with Null.
		{nil, nil, []Expr{DNull}, DNull, nil},
		{nil, nil, []Expr{DNull, DNull}, DNull, nil},
		{nil, nil, []Expr{DNull, intConst("1")}, TypeInt, nil},
		{nil, nil, []Expr{DNull, floatConst("1")}, TypeFloat, nil},
		{nil, nil, []Expr{DNull, NewDInt(1)}, TypeInt, nil},
		{nil, nil, []Expr{DNull, NewDFloat(1)}, TypeFloat, nil},
		{nil, nil, []Expr{DNull, NewDFloat(1), intConst("1")}, TypeFloat, nil},
		{nil, nil, []Expr{DNull, NewDFloat(1), floatConst("1")}, TypeFloat, nil},
		{nil, nil, []Expr{DNull, NewDFloat(1), floatConst("1")}, TypeFloat, nil},
		{nil, nil, []Expr{DNull, intConst("1"), floatConst("1")}, TypeFloat, nil},
		// Verify desired type when possible.
		{nil, TypeInt, []Expr{intConst("1")}, TypeInt, nil},
		{nil, TypeInt, []Expr{NewDInt(1)}, TypeInt, nil},
		{nil, TypeInt, []Expr{floatConst("1")}, TypeInt, nil},
		{nil, TypeInt, []Expr{NewDFloat(1)}, TypeFloat, nil},
		{nil, TypeFloat, []Expr{intConst("1")}, TypeFloat, nil},
		{nil, TypeFloat, []Expr{NewDInt(1)}, TypeInt, nil},
		{nil, TypeInt, []Expr{intConst("1"), floatConst("1")}, TypeInt, nil},
		{nil, TypeInt, []Expr{intConst("1"), floatConst("1.1")}, TypeFloat, nil},
		{nil, TypeFloat, []Expr{intConst("1"), floatConst("1")}, TypeFloat, nil},
		// Verify desired type when possible with unresolved constants.
		{nil, TypeFloat, []Expr{ValArg{"a"}}, TypeFloat, mapArgsFloat},
		{nil, TypeFloat, []Expr{intConst("1"), ValArg{"a"}}, TypeFloat, mapArgsFloat},
		{nil, TypeFloat, []Expr{floatConst("1"), ValArg{"a"}}, TypeFloat, mapArgsFloat},
	}
	for i, d := range testData {
		if d.expectedArgs == nil {
			d.expectedArgs = make(MapArgs)
		}
		forEachPerm(d.exprs, 0, func(exprs []Expr) {
			args := cloneMapArgs(d.args)
			_, typ, err := typeCheckSameTypedExprs(args, d.desired, exprs...)
			if err != nil {
				t.Errorf("%d: unexpected error returned from typeCheckSameTypedExprs: %v", i, err)
			} else {
				if !typ.TypeEqual(d.expectedType) {
					t.Errorf("%d: expected type %s when type checking %s, found %s", i, d.expectedType.Type(), exprs, typ.Type())
				}
				if !reflect.DeepEqual(args, d.expectedArgs) {
					t.Errorf("%d: expected args %v after typeCheckSameTypedExprs for %v, found %v", i, d.expectedArgs, exprs, args)
				}
			}
		})
	}
}

func TestTypeCheckSameTypedExprsError(t *testing.T) {
	floatIntMismatchErr := `expected .* to be of type (float|int), found type (float|int)`
	paramErr := `could not determine data type of parameter .*`

	testData := []struct {
		args        MapArgs
		desired     Datum
		exprs       []Expr
		expectedErr string
	}{
		{nil, nil, []Expr{NewDInt(1), floatConst("1.1")}, floatIntMismatchErr},
		{nil, nil, []Expr{NewDInt(1), NewDFloat(1)}, floatIntMismatchErr},
		{MapArgs{"a": TypeInt}, nil, []Expr{NewDFloat(1.1), ValArg{"a"}}, floatIntMismatchErr},
		{MapArgs{"a": TypeInt}, nil, []Expr{floatConst("1.1"), ValArg{"a"}}, floatIntMismatchErr},
		{MapArgs{"a": TypeFloat, "b": TypeInt}, nil, []Expr{ValArg{"b"}, ValArg{"a"}}, floatIntMismatchErr},
		{nil, nil, []Expr{ValArg{"b"}, ValArg{"a"}}, paramErr},
	}
	for i, d := range testData {
		forEachPerm(d.exprs, 0, func(exprs []Expr) {
			if _, _, err := typeCheckSameTypedExprs(d.args, d.desired, exprs...); !testutils.IsError(err, d.expectedErr) {
				t.Errorf("%d: expected %s, but found %v", i, d.expectedErr, err)
			}
		})
	}
}
