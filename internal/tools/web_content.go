package tools

import (
	"fmt"
	"mime"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func transformFetchedContent(raw []byte, contentType, format string) (string, error) {
	mediaType := normalizeMediaType(contentType)
	text := string(raw)

	if strings.EqualFold(mediaType, "text/html") {
		cleanedHTML, cleanedText, err := extractHTMLContent(text)
		if err != nil {
			return "", err
		}
		if strings.EqualFold(format, "markdown") {
			return htmlFragmentToMarkdown(cleanedHTML), nil
		}
		return cleanedText, nil
	}

	if !isTextualContentType(mediaType) {
		return "", fmt.Errorf("unsupported content type for web fetch: %s", mediaType)
	}

	return text, nil
}

func normalizeMediaType(contentType string) string {
	if contentType == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(contentType))
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func isTextualContentType(mediaType string) bool {
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	return mediaType == "application/json" ||
		mediaType == "application/xml" ||
		mediaType == "application/javascript" ||
		mediaType == "application/x-www-form-urlencoded" ||
		strings.HasSuffix(mediaType, "+json") ||
		strings.HasSuffix(mediaType, "+xml")
}

func extractHTMLContent(rawHTML string) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return "", "", err
	}

	doc.Find("script,style,noscript,template,svg").Each(func(_ int, sel *goquery.Selection) {
		sel.Remove()
	})

	root := pickPrimaryHTMLSelection(doc)
	if root.Length() == 0 {
		root = doc.Selection
	}

	htmlFragment, err := root.Html()
	if err != nil || strings.TrimSpace(htmlFragment) == "" {
		htmlFragment = rawHTML
	}

	text := normalizeWhitespace(root.Text())
	if text == "" {
		text = normalizeWhitespace(doc.Text())
	}

	return htmlFragment, text, nil
}

func pickPrimaryHTMLSelection(doc *goquery.Document) *goquery.Selection {
	candidates := []*goquery.Selection{
		doc.Find("main").First(),
		doc.Find("article").First(),
		doc.Find("body").First(),
	}

	var best *goquery.Selection
	bestLen := 0
	for _, candidate := range candidates {
		if candidate == nil || candidate.Length() == 0 {
			continue
		}
		textLen := len(normalizeWhitespace(candidate.Text()))
		if textLen > bestLen {
			best = candidate
			bestLen = textLen
		}
	}
	if best != nil {
		return best
	}
	return doc.Selection
}

func htmlFragmentToMarkdown(fragment string) string {
	doc, err := html.Parse(strings.NewReader("<html><body>" + fragment + "</body></html>"))
	if err != nil {
		return normalizeWhitespace(fragment)
	}
	body := findHTMLNode(doc, "body")
	if body == nil {
		return normalizeWhitespace(fragment)
	}

	var b strings.Builder
	renderMarkdownChildren(&b, body)
	return cleanMarkdownSpacing(b.String())
}

func findHTMLNode(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findHTMLNode(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func renderMarkdownChildren(b *strings.Builder, n *html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		renderMarkdownNode(b, child)
	}
}

func renderMarkdownNode(b *strings.Builder, n *html.Node) {
	if n == nil {
		return
	}

	switch n.Type {
	case html.TextNode:
		text := normalizeTextNodeData(n.Data)
		if text != "" {
			b.WriteString(text)
		}
	case html.ElementNode:
		switch n.Data {
		case "html", "body", "section", "article", "main", "div":
			renderMarkdownChildren(b, n)
			ensureParagraphBreak(b)
		case "h1", "h2", "h3", "h4", "h5", "h6":
			level := int(n.Data[1] - '0')
			text := strings.TrimSpace(renderInlineText(n))
			if text == "" {
				return
			}
			ensureParagraphBreak(b)
			b.WriteString(strings.Repeat("#", level))
			b.WriteString(" ")
			b.WriteString(text)
			b.WriteString("\n\n")
		case "p":
			text := strings.TrimSpace(renderInlineText(n))
			if text == "" {
				return
			}
			ensureParagraphBreak(b)
			b.WriteString(text)
			b.WriteString("\n\n")
		case "ul":
			ensureParagraphBreak(b)
			renderList(b, n, false)
			b.WriteString("\n")
		case "ol":
			ensureParagraphBreak(b)
			renderList(b, n, true)
			b.WriteString("\n")
		case "pre":
			code := strings.TrimSpace(extractCodeText(n))
			if code == "" {
				return
			}
			ensureParagraphBreak(b)
			b.WriteString("```text\n")
			b.WriteString(code)
			b.WriteString("\n```\n\n")
		case "blockquote":
			text := strings.TrimSpace(renderInlineText(n))
			if text == "" {
				return
			}
			ensureParagraphBreak(b)
			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				b.WriteString("> ")
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		case "br":
			b.WriteString("\n")
		case "table":
			text := strings.TrimSpace(normalizeWhitespace(goquery.NewDocumentFromNode(n).Text()))
			if text == "" {
				return
			}
			ensureParagraphBreak(b)
			b.WriteString(text)
			b.WriteString("\n\n")
		default:
			renderMarkdownChildren(b, n)
		}
	}
}

func renderList(b *strings.Builder, n *html.Node, ordered bool) {
	index := 1
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.Data != "li" {
			continue
		}
		item := strings.TrimSpace(renderInlineText(child))
		if item == "" {
			continue
		}
		if ordered {
			b.WriteString(fmt.Sprintf("%d. %s\n", index, item))
			index++
			continue
		}
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
}

func renderInlineText(n *html.Node) string {
	if n == nil {
		return ""
	}

	var b strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			text := normalizeTextNodeData(child.Data)
			if text != "" {
				b.WriteString(text)
			}
		case html.ElementNode:
			switch child.Data {
			case "strong", "b":
				text := strings.TrimSpace(renderInlineText(child))
				if text != "" {
					b.WriteString("**")
					b.WriteString(text)
					b.WriteString("**")
				}
			case "em", "i":
				text := strings.TrimSpace(renderInlineText(child))
				if text != "" {
					b.WriteString("*")
					b.WriteString(text)
					b.WriteString("*")
				}
			case "code":
				text := strings.TrimSpace(extractCodeText(child))
				if text != "" {
					b.WriteString("`")
					b.WriteString(text)
					b.WriteString("`")
				}
			case "a":
				text := strings.TrimSpace(renderInlineText(child))
				href := strings.TrimSpace(htmlAttr(child, "href"))
				switch {
				case text == "" && href != "":
					b.WriteString(href)
				case href == "" || href == text:
					b.WriteString(text)
				default:
					b.WriteString(text)
					b.WriteString(" (")
					b.WriteString(href)
					b.WriteString(")")
				}
			case "br":
				b.WriteString("\n")
			case "img":
				alt := strings.TrimSpace(htmlAttr(child, "alt"))
				src := strings.TrimSpace(htmlAttr(child, "src"))
				if alt != "" {
					b.WriteString(alt)
				} else if src != "" {
					b.WriteString(src)
				}
			default:
				b.WriteString(renderInlineText(child))
			}
		}
	}
	return normalizeInlineMarkdown(b.String())
}

func extractCodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func htmlAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func ensureParagraphBreak(b *strings.Builder) {
	current := b.String()
	if current == "" {
		return
	}
	if strings.HasSuffix(current, "\n\n") {
		return
	}
	if strings.HasSuffix(current, "\n") {
		b.WriteString("\n")
		return
	}
	b.WriteString("\n\n")
}

func normalizeWhitespace(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		line = normalizeInlineWhitespace(line)
		if line == "" {
			if lastBlank {
				continue
			}
			lastBlank = true
			cleaned = append(cleaned, "")
			continue
		}
		lastBlank = false
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func normalizeInlineWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\u00a0", " ")), " ")
}

func normalizeTextNodeData(s string) string {
	normalized := strings.ReplaceAll(s, "\u00a0", " ")
	hasLeading := len(normalized) > 0 && strings.TrimLeft(normalized, " \t\r\n") != normalized
	hasTrailing := len(normalized) > 0 && strings.TrimRight(normalized, " \t\r\n") != normalized
	core := strings.Join(strings.Fields(normalized), " ")
	if core == "" {
		if hasLeading || hasTrailing {
			return " "
		}
		return ""
	}
	if hasLeading {
		core = " " + core
	}
	if hasTrailing {
		core += " "
	}
	return core
}

func normalizeInlineMarkdown(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	replacer := strings.NewReplacer(
		" .", ".",
		" ,", ",",
		" !", "!",
		" ?", "?",
		" ;", ";",
		" :", ":",
	)
	return replacer.Replace(s)
}

func cleanMarkdownSpacing(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	blankCount := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			blankCount++
			if blankCount > 2 {
				continue
			}
			out = append(out, "")
			continue
		}
		blankCount = 0
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
