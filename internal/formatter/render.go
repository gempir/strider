//strider:ignore-file cognitive-complexity,cyclomatic-complexity,function-length,modifies-parameter,redefines-builtin-id,single-case-switch
package formatter

import (
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

type group struct {
	close  int
	broken bool
}

func renderWithModule(tree *cst.Tree, options Options, module string) string {
	layout := newLayout(tree, module)
	writer := writer{
		lineStart:     true,
		maxEmptyLines: 1,
	}
	source := tree.Bytes()
	writer.output.Grow(len(source))
	comments := tree.Comments()
	commentIndex := 0
	groups := []group{}
	previous := token.ILLEGAL
	sourceEnd := 0

	for index := 0; index < len(layout.tokens); index++ {
		current := layout.tokens[index]
		currentLayout := layout.tokenLayout[index]
		if current.Ch() == token.EOF {
			layout.renderCommentsBefore(&writer, comments, &commentIndex, source, &sourceEnd, len(source)+1)
			break
		}
		position := current.Position()
		layout.renderCommentsBefore(&writer, comments, &commentIndex, source, &sourceEnd, position.Offset)

		if layout.importStart >= 0 && index == layout.importStart {
			layout.renderImports(&writer)
			index = layout.importEnd
			last := layout.tokens[layout.importEnd]
			sourceEnd = max(sourceEnd, tokenSourceEnd(last))
			previous = token.SEMICOLON
			continue
		}
		if layout.importStart >= 0 && index > layout.importStart && index <= layout.importEnd {
			continue
		}

		kind := current.Ch()
		switch {
		case kind == token.SEMICOLON:
			if currentLayout.softSemi {
				writer.write(";", -1)
				writer.forceSpace = true
			} else if currentLayout.topSemi {
				writer.requestNewlines(2)
			} else {
				writer.requestNewlines(1)
			}
		case currentLayout.hardOpen != 0:
			writer.beforeToken(previous, kind, true)
			writer.write(current.Src(), -1)
			if currentLayout.hardOpen != index+1 {
				writer.indent++
				writer.requestNewlines(1)
			}
		case currentLayout.hardClose:
			if index > 0 && layout.tokenLayout[index-1].hardOpen != index {
				writer.indent = max(0, writer.indent-1)
				writer.requestNewlines(1)
			}
			writer.write(current.Src(), -1)
		case currentLayout.softOpen != 0:
			if currentLayout.spaceBefore {
				writer.beforeToken(previous, kind, true)
			}
			writer.write(current.Src(), -1)
			close := currentLayout.softOpen
			broken := close != index+1 && (currentLayout.forceBreak || layout.shouldBreak(index, close, writer.column, options.PrintWidth, comments, source, true))
			groups = append(groups, group{
				close:  close,
				broken: broken,
			})
			if broken && close != index+1 {
				writer.indent++
				writer.requestNewlines(1)
			}
		case currentLayout.softClose:
			group := group{}
			if len(groups) != 0 {
				group = groups[len(groups)-1]
				groups = groups[:len(groups)-1]
			}
			if group.broken && group.close == index && previous != token.COMMA {
				writer.write(",", -1)
			}
			if group.broken && group.close == index {
				writer.indent = max(0, writer.indent-1)
				writer.requestNewlines(1)
			}
			writer.write(current.Src(), -1)
		case kind == token.COMMA:
			group := group{}
			if len(groups) != 0 && currentLayout.commaClose == groups[len(groups)-1].close {
				group = groups[len(groups)-1]
			}
			if group.close != 0 && !group.broken && group.close == index+1 {
				break
			}
			writer.write(",", -1)
			if group.broken {
				writer.requestNewlines(1)
			} else {
				writer.forceSpace = true
			}
		case currentLayout.caseToken:
			writer.requestNewlines(1)
			writer.write(current.Src(), max(0, writer.indent-1))
			writer.forceSpace = kind == token.CASE
		case currentLayout.caseColon:
			writer.write(":", -1)
			writer.requestNewlines(1)
		case currentLayout.labelColon:
			writer.write(":", -1)
			writer.requestNewlines(1)
		case currentLayout.channelArrow:
			if previous == token.CHAN {
				writer.write(current.Src(), -1)
				writer.forceSpace = true
			} else {
				writer.beforeToken(previous, kind, true)
				writer.write(current.Src(), -1)
			}
		case currentLayout.spacedOp:
			writer.beforeToken(previous, kind, true)
			writer.write(current.Src(), -1)
			writer.forceSpace = true
		case currentLayout.unaryOp:
			writer.beforeToken(previous, kind, previous.IsKeyword())
			writer.write(current.Src(), -1)
		case kind == token.COLON:
			writer.write(":", -1)
			writer.forceSpace = currentLayout.spacedAfter
		case kind == token.PERIOD:
			writer.write(".", -1)
		default:
			writer.beforeToken(previous, kind, currentLayout.spaceBefore)
			writer.write(current.Src(), -1)
		}
		previous = kind
		sourceEnd = max(sourceEnd, tokenSourceEnd(current))
	}

	result := strings.TrimRight(writer.output.String(), " \t\r\n") + "\n"
	return result
}

func tokenSourceEnd(current cst.Token) int {
	end := current.Position().Offset
	if current.Ch() != token.SEMICOLON || current.Src() == ";" {
		end += len(current.Src())
	}
	return end
}

func (l *layout) renderCommentsBefore(writer *writer, comments []cst.Comment, commentIndex *int, source []byte, sourceEnd *int, beforeOffset int) {
	previousBuildConstraint := false
	for *commentIndex < len(comments) && comments[*commentIndex].Start < beforeOffset {
		comment := comments[*commentIndex]
		inline := *sourceEnd > 0 && countNewlines(source, *sourceEnd, comment.Start) == 0
		if inline {
			writer.pendingNewlines = 0
			writer.forceSpace = true
		} else if writer.output.Len() != 0 {
			writer.requestNewlines(1)
		}
		writer.preserveEmptyLines(source, *sourceEnd, comment.Start, previousBuildConstraint)
		writer.write(normalizeLineComment(comment.Text), -1)
		inlineBlock := inline && strings.HasPrefix(comment.Text, "/*") && !strings.Contains(comment.Text, "\n") && countNewlines(source, comment.End, beforeOffset) == 0
		if inlineBlock {
			writer.forceSpace = true
		} else {
			writer.requestNewlines(1)
		}
		*sourceEnd = comment.End
		previousBuildConstraint = isBuildConstraint(comment.Text)
		*commentIndex++
	}
	writer.preserveEmptyLines(source, *sourceEnd, beforeOffset, previousBuildConstraint)
}

func countNewlines(source []byte, start, end int) int {
	start = max(0, min(start, len(source)))
	end = max(start, min(end, len(source)))
	count := 0
	for _, current := range source[start:end] {
		if current == '\n' {
			count++
		}
	}
	return count
}

func isBuildConstraint(comment string) bool {
	return strings.HasPrefix(comment, "//go:build") || strings.HasPrefix(comment, "// +build")
}

func normalizeLineComment(comment string) string {
	if !strings.HasPrefix(comment, "//") || len(comment) <= 2 || comment[2] == ' ' || comment[2] == '\t' {
		return comment
	}
	body := comment[2:]
	for _, prefix := range []string{
		"go:",
		"line ",
		"+build",
		"nolint",
		"strider:",
		"Code generated",
		"TODO",
		"FIXME",
		"#",
	} {
		if strings.HasPrefix(body, prefix) {
			return comment
		}
	}
	return "// " + body
}
