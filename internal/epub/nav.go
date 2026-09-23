package epub

import (
	"bytes"
	"encoding/xml"
	"strings"

	"golang.org/x/net/html"
)

// ---------- EPUB 3 navigation document ----------

func parseNavDoc(data []byte, navDocPath string) ([]*TOCItem, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	nav := findTOCNav(doc)
	if nav == nil {
		return nil, errNoTOCNav
	}
	list := firstList(nav)
	if list == nil {
		return nil, errNoTOCNav
	}
	items := parseNavList(list, navDocPath)
	if len(items) == 0 {
		return nil, errNoTOCNav
	}
	return items, nil
}

var errNoTOCNav = errMsg("no toc nav found")

type errMsg string

func (e errMsg) Error() string { return string(e) }

func findTOCNav(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "nav" {
		for _, a := range n.Attr {
			k := strings.ToLower(a.Key)
			v := strings.ToLower(a.Val)
			if (k == "epub:type" || k == "type" || k == "role") &&
				(strings.Contains(v, "toc") || strings.Contains(v, "doc-toc")) {
				// EPUB3 uses epub:type="toc"; be liberal.
				if strings.Contains(v, "toc") {
					return n
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findTOCNav(c); found != nil {
			return found
		}
	}
	return nil
}

func firstList(nav *html.Node) *html.Node {
	var q []*html.Node
	q = append(q, nav)
	for len(q) > 0 {
		n := q[0]
		q = q[1:]
		if n.Type == html.ElementNode && (n.Data == "ol" || n.Data == "ul") {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			q = append(q, c)
		}
	}
	return nil
}

func parseNavList(list *html.Node, navDocPath string) []*TOCItem {
	var out []*TOCItem
	for li := list.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		var link *html.Node
		var sub *html.Node
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if c.Data == "a" && link == nil {
				link = c
			}
			if (c.Data == "ol" || c.Data == "ul") && sub == nil {
				sub = c
			}
		}
		label := ""
		href := ""
		if link != nil {
			label = strings.TrimSpace(textOf(link))
			for _, a := range link.Attr {
				if strings.EqualFold(a.Key, "href") {
					href = strings.TrimSpace(a.Val)
				}
			}
		} else {
			// span without link: group heading; label from direct text.
			label = strings.TrimSpace(textOf(li))
			if sub != nil {
				label = strings.TrimSpace(textBefore(li, sub))
			}
		}
		if label == "" {
			label = "(untitled)"
		}
		it := &TOCItem{Label: label}
		if href != "" {
			p, frag := splitHref(navDocPath, href)
			if p != "" {
				it.TargetPath = p
				it.Fragment = frag
				it.TargetHref = href
			}
		}
		if sub != nil {
			it.Children = parseNavList(sub, navDocPath)
		}
		// Keep parents with children even when unlinked.
		if it.TargetPath == "" && len(it.Children) == 0 {
			continue
		}
		out = append(out, it)
	}
	return out
}

func textOf(n *html.Node) string {
	var sb strings.Builder
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		if x.Type == html.TextNode {
			sb.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return collapseWS(sb.String())
}

func textBefore(li, stop *html.Node) string {
	var sb strings.Builder
	for c := li.FirstChild; c != nil && c != stop; c = c.NextSibling {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		} else if c.Type == html.ElementNode && c.Data != "ol" && c.Data != "ul" {
			sb.WriteString(textOf(c))
		}
	}
	return collapseWS(sb.String())
}

// ---------- EPUB 2 NCX ----------

type ncxDoc struct {
	NavMap struct {
		Points []ncxPoint `xml:"navPoint"`
	} `xml:"navMap"`
}

type ncxPoint struct {
	ID      string `xml:"id,attr"`
	Label   string `xml:"navLabel>text"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Children []ncxPoint `xml:"navPoint"`
}

func parseNCX(data []byte, ncxPath string) ([]*TOCItem, error) {
	var d ncxDoc
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	if err := dec.Decode(&d); err != nil {
		return nil, err
	}
	if len(d.NavMap.Points) == 0 {
		return nil, errMsg("empty navMap")
	}
	var conv func(ps []ncxPoint) []*TOCItem
	conv = func(ps []ncxPoint) []*TOCItem {
		var out []*TOCItem
		for _, p := range ps {
			label := strings.TrimSpace(p.Label)
			if label == "" {
				label = "(untitled)"
			}
			it := &TOCItem{Label: label}
			if src := strings.TrimSpace(p.Content.Src); src != "" {
				pp, frag := splitHref(ncxPath, src)
				if pp != "" {
					it.TargetPath = pp
					it.Fragment = frag
					it.TargetHref = src
				}
			}
			it.Children = conv(p.Children)
			if it.TargetPath == "" && len(it.Children) == 0 {
				continue
			}
			out = append(out, it)
		}
		return out
	}
	return conv(d.NavMap.Points), nil
}

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
