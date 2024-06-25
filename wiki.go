package main

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

type Page struct {
	Title    string
	Body     []byte
	HTMLBody template.HTML
}

var templates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html", "tmpl/wiki_link.html", "tmpl/all.html"))
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")
var wikiLink = regexp.MustCompile(`\[\[([a-zA-Z0-9]+)\]\]`)
var externalLink = regexp.MustCompile(`\[(https?://[^\s]+)\s([^\]]+)\]`)

func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func htmlLink(href string, text string) []byte {
	return []byte("<a href=\"" + href + "\">" + text + "</a>")
}

func wikiLinkToHTML(link []byte) []byte {
	matches := wikiLink.FindSubmatch(link)
	if matches == nil {
		return link
	}
	linkText := string(matches[1])
	htmlLink := htmlLink("/view/"+linkText, linkText)
	return []byte(template.HTML(htmlLink))
}

func externalLinkToHTML(link []byte) []byte {
	matches := externalLink.FindSubmatch(link)
	if matches == nil {
		return link
	}
	linkHref := string(matches[1])
	linkText := string(matches[2])
	htmlLink := htmlLink(linkHref, linkText)
	return []byte(template.HTML(htmlLink))
}

func renderWikiLinks(body []byte) []byte {
	body = wikiLink.ReplaceAllFunc(body, wikiLinkToHTML)
	body = externalLink.ReplaceAllFunc(body, externalLinkToHTML)
	return body
}

func wrapParagraphs(body template.HTML) template.HTML {
	lines := strings.Split(string(body), "\n")
	var paragraphs []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			paragraphs = append(paragraphs, "<p>"+line+"</p>")
		}
	}
	return template.HTML(strings.Join(paragraphs, "\n"))
}

func processBody(body []byte) template.HTML {
	body = renderWikiLinks(body)
	return wrapParagraphs(template.HTML(body))
}

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("invalid Page Title")
	}
	return m[2], nil // The title is the second subexpression.
}

func loadPage(title string) (*Page, error) {
	filename := "data/" + title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	p.HTMLBody = processBody(p.Body)
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Cannot parse form", http.StatusInternalServerError)
		return
	}
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err = p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/view/FrontPage", http.StatusFound)
}

func allHandler(w http.ResponseWriter, r *http.Request) {
	files, err := ioutil.ReadDir("data")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var titles []string
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".txt" {
			title := strings.TrimSuffix(file.Name(), ".txt")
			titles = append(titles, title)
		}
	}
	err = templates.ExecuteTemplate(w, "all.html", titles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/all", allHandler)
	http.HandleFunc("/", homeHandler)
	fmt.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
