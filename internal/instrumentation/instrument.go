package instrumentation

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"
)

// hitFunc is injected into parse trees so the real render records coverage,
// with the dot, variables and include arguments the template actually ran with.
const hitFunc = "__helmcovHit"

func hitFuncFor(trace *Trace) func(string) string {
	return func(key string) string {
		if _, ok := trace.Lines[key]; ok {
			trace.Lines[key]++
		} else if _, ok := trace.Branches[key]; ok {
			trace.Branches[key]++
		}
		return ""
	}
}

// instrumentTemplates adds hit markers to the target template and to every
// define, and registers their lines and branches at zero. The top level of a
// helper file never renders, so it is left out.
func instrumentTemplates(root *template.Template, target string, sources map[string]string, trace *Trace) {
	for _, tmpl := range root.Templates() {
		tree := tmpl.Tree
		if tree == nil || tree.Root == nil {
			continue
		}
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
	nodes := make([]parse.Node, 0, 2*len(list.Nodes))
	for _, node := range list.Nodes {
		for _, line := range in.coverableLines(node) {
			nodes = append(nodes, in.marker(node.Position(), in.registerLine(line)))
		}
		switch n := node.(type) {
		case *parse.IfNode:
			in.branch(&n.BranchNode, "if", "true", "false")
		case *parse.RangeNode:
			in.branch(&n.BranchNode, "range", "non-empty", "empty")
		case *parse.WithNode:
			in.branch(&n.BranchNode, "with", "non-empty", "empty")
		}
		nodes = append(nodes, node)
	}
	list.Nodes = nodes
}

func (in *instrumenter) branch(b *parse.BranchNode, construct, taken, notTaken string) {
	line := in.line(b.Position())
	takenKey := in.registerBranch(line, construct, taken)
	notTakenKey := in.registerBranch(line, construct, notTaken)

	in.list(b.List)
	in.list(b.ElseList)
	b.List.Nodes = append([]parse.Node{in.marker(b.Position(), takenKey)}, b.List.Nodes...)
	// An else list that only holds a marker renders nothing, so adding one is safe.
	if b.ElseList == nil {
		b.ElseList = &parse.ListNode{NodeType: parse.NodeList, Pos: b.Position()}
	}
	b.ElseList.Nodes = append([]parse.Node{in.marker(b.Position(), notTakenKey)}, b.ElseList.Nodes...)
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

func (in *instrumenter) marker(pos parse.Pos, key string) *parse.ActionNode {
	call := &parse.CommandNode{
		NodeType: parse.NodeCommand,
		Pos:      pos,
		Args: []parse.Node{
			parse.NewIdentifier(hitFunc).SetTree(in.tree).SetPos(pos),
			&parse.StringNode{NodeType: parse.NodeString, Pos: pos, Quoted: strconv.Quote(key), Text: key},
		},
	}
	return &parse.ActionNode{
		NodeType: parse.NodeAction,
		Pos:      pos,
		Line:     in.line(pos),
		Pipe:     &parse.PipeNode{NodeType: parse.NodePipe, Pos: pos, Cmds: []*parse.CommandNode{call}},
	}
}
