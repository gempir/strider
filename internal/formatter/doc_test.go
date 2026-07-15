package formatter

import "testing"

func TestRenderGroupBreaksAtWidth(t *testing.T) {
	doc := Group{Doc: concat(
		text("call("),
		Indent{Doc: concat(softBreak(), join(concat(text(","), soft()), []Doc{text("alpha"), text("beta"), text("gamma")}))},
		IfBreak{Broken: text(",")}, softBreak(), text(")"),
	)}

	if got, want := Render(doc, 80), "call(alpha, beta, gamma)"; got != want {
		t.Fatalf("wide render:\n got %q\nwant %q", got, want)
	}
	if got, want := Render(doc, 12), "call(\n\talpha,\n\tbeta,\n\tgamma,\n)"; got != want {
		t.Fatalf("narrow render:\n got %q\nwant %q", got, want)
	}
}
