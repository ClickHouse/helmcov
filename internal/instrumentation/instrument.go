package instrumentation

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/Masterminds/sprig/v3"
)

// These functions are injected into parse trees so the real render records
// coverage, with the dot, variables and include arguments the template ran with.
const (
	hitFunc      = "__helmcovHit"
	condFunc     = "__helmcovCond"
	defaultFunc  = "__helmcovDefault"
	ternaryFunc  = "__helmcovTernary"
	coalesceFunc = "__helmcovCoalesce"
)

// coverageFuncs implements the injected functions. The value functions keep
// Sprig's behaviour and only record which outcome happened.
func coverageFuncs(trace *Trace) template.FuncMap {
	sprigFuncs := sprig.TxtFuncMap()
	empty := sprigFuncs["empty"].(func(any) bool)
	sprigDefault := sprigFuncs["default"].(func(any, ...any) any)
	hit := func(key string) {
		if _, ok := trace.Lines[key]; ok {
			trace.Lines[key]++
		} else if _, ok := trace.Branches[key]; ok {
			trace.Branches[key]++
		}
	}
	outcome := func(key string, ok bool, yes, no string) {
		if ok {
			hit(key + ":" + yes)
		} else {
			hit(key + ":" + no)
		}
	}
	return template.FuncMap{
		hitFunc: func(key string) string {
			hit(key)
			return ""
		},
		// reflect.Value in and out passes the operand to and/or untouched.
		condFunc: func(key string, v reflect.Value) reflect.Value {
			truth := v.IsValid()
			if truth {
				truth, _ = template.IsTrue(v.Interface())
			}
			outcome(key, truth, "true", "false")
			return v
		},
		defaultFunc: func(key string, d any, given ...any) any {
			outcome(key, len(given) > 0 && !empty(given[0]), "value", "fallback")
			return sprigDefault(d, given...)
		},
		ternaryFunc: func(key string, vt, vf any, cond bool) any {
			outcome(key, cond, "true", "false")
			if cond {
				return vt
			}
			return vf
		},
		coalesceFunc: func(key string, values ...any) any {
			for i, v := range values {
				if !empty(v) {
					hit(fmt.Sprintf("%s(%d):used", key, i+1))
					return v
				}
			}
			return nil
		},
	}
}

// instrumentTemplates adds hit markers to the target template and to every
// define, and registers their lines and branches at zero. The top level of a
// helper file never renders, so it is left out.
func instrumentTemplates(root *template.Template, target string, sources map[string]string, trace *Trace) {
	for _, tmpl := range root.Templates() {
		// Templates() only returns parsed templates, so Tree is always set.
		tree := tmpl.Tree
		isDefine := tmpl.Name() != tree.ParseName
		if !isDefine && tmpl.Name() != target {
			continue
		}
		instrumentTree(tree, tree.ParseName, sources[tree.ParseName], trace)
	}
}

func instrumentTree(tree *parse.Tree, file, src string, trace *Trace) {
	in := instrumenter{tree: tree, file: file, src: src, trace: trace}
	in.list(tree.Root)
}

type instrumenter struct {
	tree  *parse.Tree
	file  string
	src   string
	trace *Trace
}

// list rewrites a list in place, putting a line marker before each node and
// branch markers at the start of every if, range and with body.
func (in *instrumenter) list(list *parse.ListNode) {
	if list == nil {
		return
	}
	nodes := make([]parse.Node, 0, len(list.Nodes))
	for _, node := range list.Nodes {
		for _, line := range in.coverableLines(node) {
			nodes = append(nodes, in.marker(node.Position(), in.registerLine(line)))
		}
		switch n := node.(type) {
		case *parse.ActionNode:
			in.pipe(n.Pipe)
		case *parse.TemplateNode:
			in.pipe(n.Pipe)
		case *parse.IfNode:
			in.branch(&n.BranchNode, in.name(n, "if"), "true", "false")
		case *parse.RangeNode:
			in.branch(&n.BranchNode, "range", "non-empty", "empty")
		case *parse.WithNode:
			in.branch(&n.BranchNode, in.name(n, "with"), "non-empty", "empty")
		}
		nodes = append(nodes, node)
	}
	list.Nodes = nodes
}

// name prefixes branches written as "else if" or "else with". The parser stores
// a branch's position just after its keyword, so the source before it shows how it was written.
func (in *instrumenter) name(node parse.Node, construct string) string {
	before := strings.TrimRight(in.src[:min(int(node.Position()), len(in.src))], " \t\r\n")
	before = strings.TrimRight(strings.TrimSuffix(before, construct), " \t\r\n")
	if strings.HasSuffix(before, "else") {
		return "else-" + construct
	}
	return construct
}

func (in *instrumenter) branch(b *parse.BranchNode, construct, taken, notTaken string) {
	line := in.line(b.Position())
	takenKey := in.registerBranch(line, construct, taken)
	notTakenKey := in.registerBranch(line, construct, notTaken)
	in.pipe(b.Pipe)

	in.list(b.List)
	in.list(b.ElseList)
	b.List.Nodes = append([]parse.Node{in.marker(b.Position(), takenKey)}, b.List.Nodes...)
	// An else list that only holds a marker renders nothing, so adding one is safe.
	if b.ElseList == nil {
		b.ElseList = &parse.ListNode{NodeType: parse.NodeList, Pos: b.Position()}
	}
	b.ElseList.Nodes = append([]parse.Node{in.marker(b.Position(), notTakenKey)}, b.ElseList.Nodes...)
}

// pipe wraps the and/or operands and the default, ternary and coalesce calls in
// a pipeline, including pipelines nested in parentheses.
func (in *instrumenter) pipe(p *parse.PipeNode) {
	if p == nil {
		return
	}
	for i, cmd := range p.Cmds {
		for _, arg := range cmd.Args {
			in.nested(arg)
		}
		ident, ok := cmd.Args[0].(*parse.IdentifierNode)
		if !ok {
			continue
		}
		line := in.line(cmd.Position())
		prefix := fmt.Sprintf("%s:%d:%s", in.file, line, ident.Ident)
		switch ident.Ident {
		case "and", "or":
			in.operands(cmd, ident.Ident, line)
		case "default":
			in.registerBranch(line, "default", "value")
			in.registerBranch(line, "default", "fallback")
			in.replace(cmd, defaultFunc, prefix)
		case "ternary":
			in.registerBranch(line, "ternary", "true")
			in.registerBranch(line, "ternary", "false")
			in.replace(cmd, ternaryFunc, prefix)
		case "coalesce":
			args := len(cmd.Args) - 1
			if i > 0 {
				args++ // the piped value is the last argument
			}
			for n := 1; n <= args; n++ {
				in.registerBranch(line, fmt.Sprintf("coalesce(%d)", n), "used")
			}
			in.replace(cmd, coalesceFunc, prefix)
		}
	}
}

func (in *instrumenter) nested(node parse.Node) {
	switch n := node.(type) {
	case *parse.PipeNode:
		in.pipe(n)
	case *parse.ChainNode:
		in.nested(n.Node)
	}
}

// operands wraps each and/or operand so it is recorded only if and/or evaluates it.
func (in *instrumenter) operands(cmd *parse.CommandNode, op string, line int) {
	for i, arg := range cmd.Args[1:] {
		switch arg.(type) {
		case *parse.NilNode, *parse.BoolNode, *parse.NumberNode, *parse.StringNode:
			continue // a literal has only one outcome, so it is not a branch
		}
		construct := fmt.Sprintf("%s(%d)", op, i+1)
		in.registerBranch(line, construct, "true")
		in.registerBranch(line, construct, "false")
		key := fmt.Sprintf("%s:%d:%s", in.file, line, construct)
		cmd.Args[i+1] = &parse.PipeNode{NodeType: parse.NodePipe, Pos: arg.Position(), Cmds: []*parse.CommandNode{in.command(arg.Position(), condFunc, key, arg)}}
	}
}

// replace swaps a Sprig call for its recording version, keeping the arguments.
func (in *instrumenter) replace(cmd *parse.CommandNode, fn, key string) {
	cmd.Args = append(in.command(cmd.Position(), fn, key).Args, cmd.Args[1:]...)
}

// registerLine adds a line key at zero hits, keeping any hits already recorded.
func (in *instrumenter) registerLine(line int) string {
	key := fmt.Sprintf("%s:%d", in.file, line)
	if _, ok := in.trace.Lines[key]; !ok {
		in.trace.Lines[key] = 0
	}
	return key
}

func (in *instrumenter) registerBranch(line int, construct, outcome string) string {
	key := fmt.Sprintf("%s:%d:%s:%s", in.file, line, construct, outcome)
	if _, ok := in.trace.Branches[key]; !ok {
		in.trace.Branches[key] = 0
	}
	return key
}

// coverableLines returns the lines a node accounts for: every non-blank line
// of a text node, or the line an action starts on.
func (in *instrumenter) coverableLines(node parse.Node) []int {
	text, ok := node.(*parse.TextNode)
	if !ok {
		return []int{in.line(node.Position())}
	}
	first := in.line(text.Position())
	var lines []int
	for i, chunk := range strings.Split(string(text.Text), "\n") {
		if strings.TrimSpace(chunk) != "" {
			lines = append(lines, first+i)
		}
	}
	return lines
}

func (in *instrumenter) line(pos parse.Pos) int {
	offset := min(max(int(pos), 0), len(in.src))
	return 1 + strings.Count(in.src[:offset], "\n")
}

func (in *instrumenter) command(pos parse.Pos, fn, key string, args ...parse.Node) *parse.CommandNode {
	return &parse.CommandNode{
		NodeType: parse.NodeCommand,
		Pos:      pos,
		Args: append([]parse.Node{
			parse.NewIdentifier(fn).SetTree(in.tree).SetPos(pos),
			&parse.StringNode{NodeType: parse.NodeString, Pos: pos, Quoted: strconv.Quote(key), Text: key},
		}, args...),
	}
}

func (in *instrumenter) marker(pos parse.Pos, key string) *parse.ActionNode {
	call := in.command(pos, hitFunc, key)
	return &parse.ActionNode{
		NodeType: parse.NodeAction,
		Pos:      pos,
		Line:     in.line(pos),
		Pipe:     &parse.PipeNode{NodeType: parse.NodePipe, Pos: pos, Cmds: []*parse.CommandNode{call}},
	}
}
