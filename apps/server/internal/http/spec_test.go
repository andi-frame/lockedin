package http

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The contract is api/openapi.yaml. These tests keep the hand-written middleware
// (error statuses, public routes) from drifting away from it.

type specDoc struct {
	Paths map[string]map[string]yaml.Node `yaml:"paths"` // path items also hold parameters and servers
	Comp  struct {
		Schemas struct {
			ErrorCode struct {
				Description string   `yaml:"description"`
				Enum        []string `yaml:"enum"`
			} `yaml:"ErrorCode"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

type specOp struct {
	OperationID string   `yaml:"operationId"`
	Tags        []string `yaml:"tags"`
	Security    *[]any   `yaml:"security"` // nil = inherits the global cookie+csrf requirement
	ProblemCode []string `yaml:"x-problem-codes"`
}

var httpMethods = map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}

func loadSpec(t *testing.T) specDoc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var doc specDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return doc
}

type operation struct {
	Method, Path string
	Op           specOp
}

// operations lists the /api/v1 operations (the ops tag lives at the origin root).
func (d specDoc) operations(t *testing.T) (out []operation) {
	t.Helper()
	for path, item := range d.Paths {
		for method, node := range item {
			if !httpMethods[method] {
				continue
			}
			var op specOp
			if err := node.Decode(&op); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			if contains(op.Tags, "ops") {
				continue
			}
			out = append(out, operation{strings.ToUpper(method), path, op})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path+out[i].Method < out[j].Path+out[j].Method })
	return out
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// statusTable parses the "| 409 | `a`, `b` |" table in the ErrorCode description.
func (d specDoc) statusTable(t *testing.T) map[string]int {
	t.Helper()
	row := regexp.MustCompile(`^\|\s*(\d{3})\s*\|(.+)\|\s*$`)
	code := regexp.MustCompile("`([^`]+)`")
	table := map[string]int{}
	for _, line := range strings.Split(d.Comp.Schemas.ErrorCode.Description, "\n") {
		m := row.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		status, _ := strconv.Atoi(m[1])
		for _, c := range code.FindAllStringSubmatch(m[2], -1) {
			if _, dup := table[c[1]]; dup {
				t.Fatalf("code %q is listed twice in the ErrorCode table", c[1])
			}
			table[c[1]] = status
		}
	}
	return table
}

func TestSpecStatusTableCoversEveryErrorCode(t *testing.T) {
	d := loadSpec(t)
	table := d.statusTable(t)
	for _, c := range d.Comp.Schemas.ErrorCode.Enum {
		if _, ok := table[c]; !ok {
			t.Errorf("ErrorCode %q is in the enum but not in the status table", c)
		}
	}
	for c := range table {
		if !contains(d.Comp.Schemas.ErrorCode.Enum, c) {
			t.Errorf("code %q is in the status table but not in the enum", c)
		}
	}
}

func TestStatusForMatchesSpecTable(t *testing.T) {
	d := loadSpec(t)
	for code, want := range d.statusTable(t) {
		got, ok := StatusFor(code)
		if !ok {
			t.Errorf("StatusFor(%q) is unknown", code)
			continue
		}
		if got != want {
			t.Errorf("StatusFor(%q) = %d, spec says %d", code, got, want)
		}
	}
}

func TestProblemCodesDeclaredPerOperationExist(t *testing.T) {
	d := loadSpec(t)
	for _, op := range d.operations(t) {
		if len(op.Op.ProblemCode) == 0 {
			t.Errorf("%s %s declares no x-problem-codes", op.Method, op.Path)
		}
		for _, c := range op.Op.ProblemCode {
			if !contains(d.Comp.Schemas.ErrorCode.Enum, c) {
				t.Errorf("%s %s declares unknown code %q", op.Method, op.Path, c)
			}
		}
	}
}

// Every code the Go services can return must be in the contract, or the web app
// would receive a code it has no message for.
func TestEveryGoErrorCodeIsInTheContract(t *testing.T) {
	d := loadSpec(t)
	re := regexp.MustCompile(`(?:Code:\s*|Error\{)"([a-z_]+(?:\.[a-z_]+)*)"`)
	for _, dir := range []string{"domain", "service", "auth"} {
		files, _ := filepath.Glob(filepath.Join("..", dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
				if !contains(d.Comp.Schemas.ErrorCode.Enum, m[1]) {
					t.Errorf("%s returns code %q which is missing from the ErrorCode enum", f, m[1])
				}
			}
		}
	}
}

func TestPublicRoutesMatchSpecSecurity(t *testing.T) {
	d := loadSpec(t)
	placeholder := regexp.MustCompile(`\{[^}]+\}`)
	for _, op := range d.operations(t) {
		path := basePath + placeholder.ReplaceAllString(op.Path, "x")
		specPublic := op.Op.Security != nil && len(*op.Op.Security) == 0
		if got := isPublic(op.Method, path); got != specPublic {
			t.Errorf("%s %s: isPublic=%v but the spec says public=%v", op.Method, op.Path, got, specPublic)
		}
	}
}
