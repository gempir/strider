package formatter

import (
	"strings"
	"unicode/utf8"
)

type Doc interface{ doc() }

type Text struct{ Value string }
type SoftLine struct{ Flat string }
type HardLine struct{}
type Concat struct{ Docs []Doc }
type Indent struct{ Doc Doc }
type Group struct{ Doc Doc }
type IfBreak struct{ Broken, Flat Doc }

func (Text) doc()     {}
func (SoftLine) doc() {}
func (HardLine) doc() {}
func (Concat) doc()   {}
func (Indent) doc()   {}
func (Group) doc()    {}
func (IfBreak) doc()  {}

func text(value string) Doc { return Text{Value: value} }
func soft() Doc             { return SoftLine{Flat: " "} }
func softBreak() Doc        { return SoftLine{} }
func hard() Doc             { return HardLine{} }

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

func Render(doc Doc, width int) string {
	var output strings.Builder
	column := 0
	pendingIndent := -1
	stack := []renderItem{{doc: doc}}

	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch node := item.doc.(type) {
		case Text:
			if pendingIndent >= 0 && node.Value != "" {
				output.WriteString(strings.Repeat("\t", pendingIndent))
				pendingIndent = -1
			}
			output.WriteString(node.Value)
			if newline := strings.LastIndexByte(node.Value, '\n'); newline >= 0 {
				column = utf8.RuneCountInString(node.Value[newline+1:])
			} else {
				column += utf8.RuneCountInString(node.Value)
			}
		case SoftLine:
			if item.flat {
				if pendingIndent >= 0 && node.Flat != "" {
					output.WriteString(strings.Repeat("\t", pendingIndent))
					pendingIndent = -1
				}
				output.WriteString(node.Flat)
				column += utf8.RuneCountInString(node.Flat)
			} else {
				output.WriteByte('\n')
				pendingIndent = item.indent
				column = item.indent * 4
			}
		case HardLine:
			output.WriteByte('\n')
			pendingIndent = item.indent
			column = item.indent * 4
		case Concat:
			for index := len(node.Docs) - 1; index >= 0; index-- {
				stack = append(stack, renderItem{doc: node.Docs[index], indent: item.indent, flat: item.flat})
			}
		case Indent:
			stack = append(stack, renderItem{doc: node.Doc, indent: item.indent + 1, flat: item.flat})
		case Group:
			candidate := renderItem{doc: node.Doc, indent: item.indent, flat: true}
			stack = append(stack, renderItem{doc: node.Doc, indent: item.indent, flat: fits(width-column, candidate, stack)})
		case IfBreak:
			selected := node.Broken
			if item.flat {
				selected = node.Flat
			}
			stack = append(stack, renderItem{doc: selected, indent: item.indent, flat: item.flat})
		}
	}
	return output.String()
}

func fits(remaining int, first renderItem, rest []renderItem) bool {
	stack := make([]renderItem, 0, len(rest)+1)
	stack = append(stack, rest...)
	stack = append(stack, first)
	for remaining >= 0 && len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch node := item.doc.(type) {
		case Text:
			if strings.Contains(node.Value, "\n") {
				return true
			}
			remaining -= utf8.RuneCountInString(node.Value)
		case SoftLine:
			if !item.flat {
				return true
			}
			remaining -= utf8.RuneCountInString(node.Flat)
		case HardLine:
			return true
		case Concat:
			for index := len(node.Docs) - 1; index >= 0; index-- {
				stack = append(stack, renderItem{doc: node.Docs[index], indent: item.indent, flat: item.flat})
			}
		case Indent:
			stack = append(stack, renderItem{doc: node.Doc, indent: item.indent + 1, flat: item.flat})
		case Group:
			stack = append(stack, renderItem{doc: node.Doc, indent: item.indent, flat: true})
		case IfBreak:
			stack = append(stack, renderItem{doc: node.Flat, indent: item.indent, flat: true})
		}
	}
	return remaining >= 0
}
