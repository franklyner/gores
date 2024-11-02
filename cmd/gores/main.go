package main

import (
	"fmt"
	"franklyner/gores/middleware"
	"html/template"
	"os"
)

func main() {
	middleware.DefaultRouter.AddHandler("/env", showEnv)
	middleware.DefaultRouter.AddHandler("/tmpl", testTmpl)
	middleware.DefaultRouter.Handle()
}

func showEnv(req middleware.Request, resp *middleware.Response) {
	env := os.Environ()
	fmt.Fprintln(resp.Body, "<b>Env</b></br>")
	for _, e := range env {
		fmt.Fprintf(resp.Body, "%s</br>", e)
	}
	fmt.Fprintln(resp.Body, "</br><b>Query</b></br>")
	for k, v := range req.Query {
		fmt.Fprintf(resp.Body, "%s=%s</br>", k, v)
	}
}

func testTmpl(req middleware.Request, resp *middleware.Response) {
	itemTmpl := `
	{{define "item"}}
				<p>
				{{.}}
			</p>
	{{end}}
	`

	tmplStr := `
	<html>
		<head>
			<title>Test Templates!</title>
		</head>
		<body>
			<H1>Test Templates!</H1>
			{{range .Query}}
				{{template "item" .}}
			{{end}}

		<body>
	</html>
	`
	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		err = fmt.Errorf("error parsing template: %w", err)
		fmt.Fprint(os.Stderr, err)
	}

	tmpl, err = tmpl.Parse(itemTmpl)
	if err != nil {
		err = fmt.Errorf("error parsing template: %w", err)
		fmt.Fprint(os.Stderr, err)
	}
	err = tmpl.Execute(resp.Body, req)
	if err != nil {
		err = fmt.Errorf("error executing template: %w", err)
		fmt.Fprint(os.Stderr, err)
	}
}
