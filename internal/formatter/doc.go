package formatter

import (
	"strings"
	"unicode/utf8"
)

type Doc interface {
	doc()
}

type Text struct {
	Value string
}

type SoftLine struct {
	Flat string
}

type HardLine struct{}

type Concat struct {
	Docs []Doc
}

type Indent struct {
	Doc Doc
}

type Group struct {
	Doc Doc
}

type IfBreak struct {
	Broken, Flat Doc
}

func (Text) doc() {}

func (SoftLine) doc() {}

func (HardLine) doc() {}

func (Concat) doc() {}

func (Indent) doc() {}

func (Group) doc() {}

func (IfBreak) doc() {}

func text(value string) Doc {
	return Text{Value: value}
}

func soft() Doc {
	return SoftLine{Flat: " "}
}

func softBreak() Doc {
	return SoftLine{}
}

func hard() Doc {
	return HardLine{}
}

func concat(docs ...Doc) Doc {
	flat := make([]Doc, 0, len(docs))
	for _, item := range docs {
		if item == nil {
			continue
		}
		if nested, ok := item.(Concat); ok {
			flat = append(flat, nested.Docs...)
		} else {
			flat = append(flat, item)
		}
	}
	return Concat{Docs: flat}
}

func join(separator Doc, docs []Doc) Doc {
	if len(docs) == 0 {
		return Text{}
	}
	result := make([]Doc, 0, len(docs)*2-1)
	for index, item := range docs {
		if index != 0 {
			result = append(result, separator)
		}
		result = append(result, item)
	}
	return concat(result...)
}

type renderItem struct {
	doc    Doc
	indent int
	flat   bool
}

type renderer struct {
	output        strings.Builder
	column        int
	pendingIndent int
	width         int
	indentWidth   int
	stack         []renderItem
}

func Render(doc Doc, width int) string {
	return RenderWithIndentWidth(doc, width, 4)
}

func RenderWithIndentWidth(doc Doc, width, indentWidth int) string {
	renderer := renderer{
		pendingIndent: -1,
		width:         width,
		indentWidth:   indentWidth,
		stack:         []renderItem{{doc: doc}},
	}
	for len(renderer.stack) > 0 {
		renderer.renderNext()
	}
	return renderer.output.String()
}

func (r *renderer) renderNext() {
	item := r.pop()
	switch node := item.doc.(type) {
	case Text:
		r.renderText(node)
	case SoftLine:
		r.renderSoftLine(item, node)
	case HardLine:
		r.newline(item.indent)
	case Concat:
		r.pushConcat(item, node.Docs)
	case Indent:
		r.stack = append(
			r.stack,
			renderItem{doc: node.Doc, indent: item.indent + 1, flat: item.flat},
		)
	case Group:
		candidate := renderItem{doc: node.Doc, indent: item.indent, flat: true}
		r.stack = append(
			r.stack,
			renderItem{
				doc:    node.Doc,
				indent: item.indent,
				flat:   fits(r.width-r.column, candidate, r.stack),
			},
		)
	case IfBreak:
		r.renderIfBreak(item, node)
	}
}

func (r *renderer) pop() renderItem {
	item := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	return item
}

func (r *renderer) renderText(node Text) {
	if r.pendingIndent >= 0 && node.Value != "" {
		r.output.WriteString(strings.Repeat("\t", r.pendingIndent))
		r.pendingIndent = -1
	}
	r.output.WriteString(node.Value)
	if newline := strings.LastIndexByte(node.Value, '\n'); newline >= 0 {
		r.column = utf8.RuneCountInString(node.Value[newline+1:])
		return
	}
	r.column += utf8.RuneCountInString(node.Value)
}

func (r *renderer) renderSoftLine(item renderItem, node SoftLine) {
	if !item.flat {
		r.newline(item.indent)
		return
	}
	if r.pendingIndent >= 0 && node.Flat != "" {
		r.output.WriteString(strings.Repeat("\t", r.pendingIndent))
		r.pendingIndent = -1
	}
	r.output.WriteString(node.Flat)
	r.column += utf8.RuneCountInString(node.Flat)
}

func (r *renderer) newline(indent int) {
	r.output.WriteByte('\n')
	r.pendingIndent = indent
	r.column = indent * r.indentWidth
}

func (r *renderer) pushConcat(item renderItem, docs []Doc) {
	for index := len(docs) - 1; index >= 0; index-- {
		r.stack = append(
			r.stack,
			renderItem{doc: docs[index], indent: item.indent, flat: item.flat},
		)
	}
}

func (r *renderer) renderIfBreak(item renderItem, node IfBreak) {
	selected := node.Broken
	if item.flat {
		selected = node.Flat
	}
	r.stack = append(r.stack, renderItem{doc: selected, indent: item.indent, flat: item.flat})
}

type fitState struct {
	remaining int
	stack     []renderItem
	boundary  int
}

func fits(remaining int, first renderItem, rest []renderItem) bool {
	state := fitState{
		remaining: remaining,
		stack:     append([]renderItem(nil), rest...),
		boundary:  len(rest),
	}
	state.stack = append(state.stack, first)
	for state.remaining >= 0 && len(state.stack) > state.boundary {
		if state.step() {
			return true
		}
	}
	return state.remaining >= 0
}

func (state *fitState) step() bool {
	item := state.pop()
	switch node := item.doc.(type) {
	case Text:
		return state.text(node)
	case SoftLine:
		return state.softLine(item, node)
	case HardLine:
		state.remaining = -1
	case Concat:
		state.pushConcat(item, node.Docs)
	case Indent:
		state.stack = append(
			state.stack,
			renderItem{doc: node.Doc, indent: item.indent + 1, flat: item.flat},
		)
	case Group:
		state.stack = append(
			state.stack,
			renderItem{doc: node.Doc, indent: item.indent, flat: true},
		)
	case IfBreak:
		state.stack = append(
			state.stack,
			renderItem{doc: node.Flat, indent: item.indent, flat: true},
		)
	}
	return false
}

func (state *fitState) pop() renderItem {
	item := state.stack[len(state.stack)-1]
	state.stack = state.stack[:len(state.stack)-1]
	return item
}

func (state *fitState) text(node Text) bool {
	if strings.Contains(node.Value, "\n") || strings.HasPrefix(node.Value, "//") {
		state.remaining = -1
		return false
	}
	state.remaining -= utf8.RuneCountInString(node.Value)
	return false
}

func (state *fitState) softLine(item renderItem, node SoftLine) bool {
	if !item.flat {
		state.remaining = -1
		return false
	}
	state.remaining -= utf8.RuneCountInString(node.Flat)
	return false
}

func (state *fitState) pushConcat(item renderItem, docs []Doc) {
	for index := len(docs) - 1; index >= 0; index-- {
		state.stack = append(
			state.stack,
			renderItem{doc: docs[index], indent: item.indent, flat: item.flat},
		)
	}
}
