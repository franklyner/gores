package main

import (
	"errors"
	"fmt"
	"franklyner/gores/app"
	"franklyner/gores/middleware"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	middleware.Initialize(middleware.ConfigImpl{
		DBHost:     "lynersc.mysql.db.internal",
		DBName:     "lynersc_frank",
		DBUser:     "lynersc_frank",
		DBPassword: "Z8wMG4iwmH-nT5cGgHVW",
		RootPath:   "/cgi-bin/gores",
	})
	middleware.DefaultRouter.AddHandler("/env", showEnv)
	middleware.DefaultRouter.AddHandler("/tmpl", testTmpl)
	middleware.DefaultRouter.AddHandler("/redir", testRedirect)
	middleware.DefaultRouter.AddHandler("/db", testDB)

	middleware.DefaultRouter.AddHandler("/login", showLogin)
	middleware.DefaultRouter.AddHandler("/logout", doLogout)
	middleware.DefaultRouter.AddHandler("/dologin", doLogin)
	middleware.DefaultRouter.AddHandler("/main", showMain)

	middleware.DefaultRouter.Handle()
}

func showLogin(req middleware.Request, resp *middleware.Response) bool {
	html := `
	<form method="POST" action="dologin">
		Username: <input type="text" name="username" /><br />
		Password: <input type="password" name="password"><br />
		<input type="submit" name="login" />
	</form>
	`
	fmt.Fprint(resp.Body, html)
	return true
}

func doLogin(req middleware.Request, resp *middleware.Response) bool {

	// Extract username and password
	username := req.Form.Get("username")
	password := req.Form.Get("password")

	user, err := app.LoadUser(username)
	log.Default().Printf("user: %+v", user)
	if err != nil {
		log.Default().Println(err)
		if errors.Is(err, app.ErrNotFound) {
			middleware.SendError(http.StatusUnauthorized, "invalid username or password")
		} else {
			middleware.SendError(http.StatusInternalServerError, err.Error())
		}
		return false
	}

	if user.Password != strings.TrimSpace(password) {
		log.Default().Println(password)
		middleware.SendError(http.StatusUnauthorized, "invalid username or password")
		return false
	}
	middleware.Session.Set("username", username)
	log.Default().Printf("set username %s to session, redirecting to main", middleware.Session.Get("username"))
	resp.SendRedirect("/main")
	return false
}

func doLogout(req middleware.Request, resp *middleware.Response) bool {
	middleware.Session.Delete()
	resp.SendRedirect("/login")
	return true
}

func showMain(req middleware.Request, resp *middleware.Response) bool {
	if !ensureAuth(resp) {
		return true
	}
	now := time.Now()
	cal, err := app.LoadCalendarForMonth(now.Year(), int(now.Month()))
	if err != nil {
		fmt.Fprintf(resp.Body, "Error loading calendar: %s\n", err.Error())
		return true
	}

	tmpl, err := template.ParseFiles("../templates/main.twig")
	if err != nil {
		fmt.Fprintf(resp.Body, "Error loading template: %s\n", err.Error())
		return true
	}

	err = tmpl.Execute(resp.Body, cal)
	if err != nil {
		fmt.Fprintf(resp.Body, "Error executing template: %s\n", err.Error())
		return true
	}

	// fmt.Fprintf(resp.Body, "Welcome: %s <br />\n", middleware.Session.Get("username"))
	// fmt.Fprintf(resp.Body, "%s <br />\n", cal.MonthYear)
	// fmt.Fprintln(resp.Body, "<table>")
	// for _, week := range cal.Weeks {
	// 	fmt.Fprintln(resp.Body, "<tr>")
	// 	for _, day := range week {
	// 		fmt.Fprintln(resp.Body, "<td>")
	// 		fmt.Fprintln(resp.Body, day.DayOfMonth)
	// 		fmt.Fprintln(resp.Body, day.Entry.User)
	// 		fmt.Fprintln(resp.Body, "</td>")

	// 	}
	// 	fmt.Fprintln(resp.Body, "</tr>")
	// }
	// fmt.Fprintln(resp.Body, "<table>")

	return true
}

func showEnv(req middleware.Request, resp *middleware.Response) bool {
	env := os.Environ()
	fmt.Fprintln(resp.Body, "<b>Env</b></br>")
	for _, e := range env {
		fmt.Fprintf(resp.Body, "%s</br>", e)
	}
	fmt.Fprintln(resp.Body, "</br><b>Query</b></br>")
	for k, v := range req.Query {
		fmt.Fprintf(resp.Body, "%s=%s</br>", k, v)
	}
	return true
}

func testRedirect(req middleware.Request, resp *middleware.Response) bool {
	resp.SendRedirect("/tmpl")
	return false
}

type SessionEntry struct {
	SessionID string
	Label     string
	Value     string
	UpdatedAt time.Time
}

func testDB(req middleware.Request, resp *middleware.Response) bool {
	if middleware.Session.Get("test") == "" {
		fmt.Fprintf(resp.Body, "test is empty, setting it")
		middleware.Session.Set("test", "oh, yeah!")
	} else {
		fmt.Fprintf(resp.Body, "test: %s", middleware.Session.Get("test"))
	}
	return true
}

func testTmpl(req middleware.Request, resp *middleware.Response) bool {
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
	return true
}

func ensureAuth(resp *middleware.Response) bool {
	username := middleware.Session.Get("username")
	if username == "" {
		resp.SendRedirect("/logout")
		return false
	}
	return true
}
