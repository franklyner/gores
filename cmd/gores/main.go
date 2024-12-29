package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"franklyner/gores/app"
	"franklyner/gores/middleware"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ConfigHost           = "db_host"
	ConfigDBName         = "db_name"
	ConfigDBUser         = "db_user"
	ConfigDBPwd          = "db_password"
	ConfigBGColor        = "bg_color"
	ConfigContentBGColor = "content_bg_color"
	ConfigTitle          = "title"
)

var config map[string]string

func main() {
	confData, err := os.ReadFile("../.goresconf")
	if err != nil {
		panic(err)
	}
	config = make(map[string]string)
	err = json.Unmarshal(confData, &config)
	if err != nil {
		panic(err)
	}
	middleware.Initialize(middleware.ConfigImpl{
		DBHost:     config[ConfigHost],
		DBName:     config[ConfigDBName],
		DBUser:     config[ConfigDBUser],
		DBPassword: config[ConfigDBPwd],
		RootPath:   "/cgi-bin/gores",
	})
	log.Default().Print("Request start")
	middleware.DefaultRouter.AddHandler("/env", showEnv)
	middleware.DefaultRouter.AddHandler("/tmpl", testTmpl)
	middleware.DefaultRouter.AddHandler("/redir", testRedirect)
	middleware.DefaultRouter.AddHandler("/db", testDB)

	middleware.DefaultRouter.AddHandler("/login", showLogin)
	middleware.DefaultRouter.AddHandler("/logout", doLogout)
	middleware.DefaultRouter.AddHandler("/dologin", doLogin)
	middleware.DefaultRouter.AddHandler("/main", showMain)
	middleware.DefaultRouter.AddHandler("/doSave", doSave)
	middleware.DefaultRouter.AddHandler("/doDelete", doDelete)

	middleware.DefaultRouter.Handle()
}

func showLogin(req middleware.Request, resp *middleware.Response) bool {
	html := `
	<form method="POST" action="/cgi-bin/gores/dologin">
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
	resp.SendRedirect("main")
	return false
}

func doLogout(req middleware.Request, resp *middleware.Response) bool {
	middleware.Session.Delete()
	resp.SendRedirect("/index.html")
	return true
}

func showMain(req middleware.Request, resp *middleware.Response) bool {
	if !ensureAuth(resp) {
		return true
	}
	mstr := req.Query.Get("m")
	ystr := req.Query.Get("y")
	var mon int
	var year int
	var err error
	if mstr == "" || ystr == "" {
		now := time.Now()
		year = now.Year()
		mon = int(now.Month())
	} else {
		mon, err = strconv.Atoi(mstr)
		if err != nil {
			log.Default().Printf("Error reading month param: %s\n", err.Error())
			return true
		}
		year, err = strconv.Atoi(ystr)
		if err != nil {
			log.Default().Printf("Error reading year param: %s\n", err.Error())
			return true
		}
	}
	log.Default().Print("m: ", mon, " y:", year)
	cal, err := app.LoadCalendarForMonth(year, mon)
	if err != nil {
		log.Default().Printf("Error loading calendar: %s\n", err.Error())
		return true
	}
	log.Default().Print("loaded calendar")

	tmpl, err := template.ParseFiles("../templates/main.twig", "../templates/tooltip.twig")
	if err != nil {
		fmt.Fprintf(resp.Body, "Error loading template: %s\n", err.Error())
		return true
	}
	data := map[string]any{
		"Cal":      cal,
		"Username": middleware.Session.Get("username"),
		"Message":  middleware.Session.Get("message"),
		"Config":   config,
	}
	middleware.Session.Set("message", "") // deleting message

	err = tmpl.Execute(resp.Body, data)
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

func doSave(req middleware.Request, resp *middleware.Response) bool {
	byear, _ := strconv.Atoi(req.Form.Get("byear"))
	bmonth, _ := strconv.Atoi(req.Form.Get("bmonth"))
	bday, _ := strconv.Atoi(req.Form.Get("bday"))
	eyear, _ := strconv.Atoi(req.Form.Get("end_year"))
	emonth, _ := strconv.Atoi(req.Form.Get("end_month"))
	eday, _ := strconv.Atoi(req.Form.Get("end_day"))

	start := time.Date(byear, time.Month(bmonth), bday, 0, 0, 0, 0, time.UTC)
	end := time.Date(eyear, time.Month(emonth), eday, 0, 0, 0, 0, time.UTC)

	username := middleware.Session.Get("username")
	e := app.Entry{
		User:        username,
		Begin:       start,
		End:         end,
		Bemerkungen: req.Form.Get("bemerkung"),
	}
	err := app.CreateEntry(e)
	if err != nil {
		log.Default().Print(err)
		if errors.Is(err, app.ErrConflict) {
			middleware.Session.Set("message", "Konflikt mit einer bestehenden Buchung!")
		} else {
			middleware.Session.Set("message", "Etwas ist beim speichern schiefgelaufen...")
		}
	}
	m := req.Form.Get("m")
	y := req.Form.Get("y")

	path := fmt.Sprintf("main?m=%s&y=%s", m, y)
	resp.SendRedirect(path)
	return true
}

func doDelete(req middleware.Request, resp *middleware.Response) bool {
	entryID, _ := strconv.Atoi(req.Query.Get("id"))
	m := req.Query.Get("m")
	y := req.Query.Get("y")
	user := middleware.Session.Get("username")
	err := app.DeleteEntry(entryID, user)
	if err != nil {
		log.Default().Print(err)
	}

	path := fmt.Sprintf("main?m=%s&y=%s", m, y)
	resp.SendRedirect(path)
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
		resp.SendRedirect("/index.html")
		return false
	}
	return true
}
