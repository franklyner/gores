package middleware

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

const (
	HandlerNotFoud      = "--notfound--"
	EnvQUERY            = "QUERY_STRING"
	EnvPATH             = "PATH_INFO"
	EnvCOOKIE           = "HTTP_COOKIE"
	SessionIDCookieName = "SID"
)

var DefaultRouter *Router
var Config ConfigImpl
var DB *sql.DB

func Initialize(config ConfigImpl) {
	f, err := os.OpenFile("gores.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	log.Default().SetOutput(f)
	Config = config
	initDB()
	router := &Router{
		handlers: make(map[string]func(Request, *Response) bool),
	}
	router.AddHandler(HandlerNotFoud, func(req Request, resp *Response) bool {
		fmt.Fprintln(resp.Body, "No handler defined for this path!")
		return true
	})

	DefaultRouter = router
}

func initDB() {
	// Capture connection properties.
	cfg := mysql.NewConfig()
	cfg.User = Config.DBUser
	cfg.Passwd = Config.DBPassword
	cfg.Net = "tcp"
	cfg.Addr = Config.DBHost
	cfg.DBName = Config.DBName
	cfg.ParseTime = true

	// Get a database handle.
	var err error
	DB, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Default().Fatal(err)
	}

	pingErr := DB.Ping()
	if pingErr != nil {
		log.Default().Fatal(pingErr)
	}
	log.Default().Println("Connected!")
}

type ConfigImpl struct {
	DBHost     string
	DBName     string
	DBUser     string
	DBPassword string
	RootPath   string
}

type Router struct {
	handlers map[string]func(Request, *Response) bool
}

func (r *Router) AddHandler(path string, handler func(Request, *Response) bool) {
	r.handlers[path] = handler
}

func (r *Router) Handle() {
	//	log.Default().Println("Start to handle")
	req, err := createRequest()
	if err != nil {
		SendError(http.StatusInternalServerError, err.Error())
		return
	}
	//	log.Default().Println("Created request")
	resp := createResponse()
	//	log.Default().Println("Created response")

	initSession()
	//	log.Default().Println("Initialized session")

	defer Session.SaveToDB()

	handler, found := r.handlers[req.Path]
	//	log.Default().Println("found handler: ", found)

	if found {
		handler(req, resp)
	} else {
		r.handlers[HandlerNotFoud](req, resp)
	}
	//	log.Default().Println("Executed handler")

	fmt.Printf("Set-Cookie: %s\n", Session.GetCoockieStr())
	//	log.Default().Println("wrote cookies")
	fmt.Println("Status: ", resp.Status)
	if resp.Location != "" {
		fmt.Println("Location: ", resp.Location)
		fmt.Println()
		return
	}
	for k, v := range resp.Headers {
		fmt.Printf("%s: %s\n", k, v)
	}
	//	log.Default().Println("wrote headers")

	fmt.Println()
	fmt.Print(resp.Body)
	//	log.Default().Println("wrote body")

}

func SendError(code int, msg string) {
	log.Default().Printf("ERROR: returned %d: %s", code, msg)
	//fmt.Printf("Status: %d\n", code)
	fmt.Println("Content-Type: text/plain")
	fmt.Println()
	fmt.Println(msg)
}

type Request struct {
	Path  string
	Query url.Values
	Form  url.Values
}

type Response struct {
	Headers  map[string]string
	Body     *bytes.Buffer
	Location string
	Status   int
}

func createRequest() (Request, error) {
	qryStr := os.Getenv(EnvQUERY)
	qry, err := url.ParseQuery(qryStr)
	if err != nil {
		return Request{}, fmt.Errorf("error parsing query (%s): %w", qryStr, err)
	}

	form, err := parseForm()
	if err != nil {
		return Request{}, fmt.Errorf("error parsing form (%s): %w", qryStr, err)
	}

	return Request{
		Path:  os.Getenv(EnvPATH),
		Query: qry,
		Form:  form,
	}, nil
}

func parseForm() (url.Values, error) {
	// Get the content length from the environment variable
	contentLengthStr := os.Getenv("CONTENT_LENGTH")
	if contentLengthStr == "" {
		return url.Values{}, nil
	}

	// Parse the content length to an integer
	contentLength, err := strconv.Atoi(contentLengthStr)
	if err != nil {
		return nil, fmt.Errorf("invalid content length: %s", err)
	}

	// Read the POST data from stdin
	postData := make([]byte, contentLength)
	_, err = io.ReadFull(os.Stdin, postData)
	if err != nil {
		return nil, fmt.Errorf("error reading POST data: %s", err)
	}

	// Parse the POST data as form values
	formValues, err := url.ParseQuery(string(postData))
	if err != nil {
		return nil, fmt.Errorf("error parsing POST data: %s", err)
	}
	return formValues, nil
}

func createResponse() *Response {
	resp := &Response{
		Headers: make(map[string]string),
		Body:    &bytes.Buffer{},
		Status:  200,
	}

	resp.Headers["Content-Type"] = "text/html"

	return resp
}

func (resp *Response) SendRedirect(target string) {
	resp.Status = 303
	resp.Location = Config.RootPath + target
}

// Session handling

var Session *SessionImpl

type SessionImpl struct {
	ID      string
	values  map[string]string
	cookies map[string]string
}

func (s *SessionImpl) Get(key string) string {
	return s.values[key]
}

func (s *SessionImpl) Set(key, value string) {
	s.values[key] = value
}

func (s *SessionImpl) GetCoockieStr() string {
	sb := strings.Builder{}
	first := true
	for k, v := range s.cookies {
		if !first {
			sb.WriteString("; ")
		}
		sb.WriteString(fmt.Sprintf("%s=%s", k, v))
	}
	return sb.String()
}

func (s *SessionImpl) Delete() {
	_, err := DB.Exec("DELETE FROM session_entries WHERE session_id=?", s.ID)
	if err != nil {
		log.Default().Println("Error deleting session entries: ", err)
	}
	s.ID = ""
}

func (s *SessionImpl) LoadFromDB() bool {
	dt30MinAgo := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	rows, err := DB.Query("SELECT label, value FROM session_entries WHERE session_id=? and updated_at > ?", s.ID, dt30MinAgo)
	if err != nil {
		log.Default().Panicln("error fetching rows from db: %s", err.Error())
		return false
	}
	if rows.Err() != nil {
		log.Default().Printf("error executing query: %s", rows.Err().Error())
		return false
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		found = true
		var label, value string
		err = rows.Scan(&label, &value)
		if err != nil {
			log.Default().Panicln("error fetching rows from db: %s", err.Error())
			return false
		}
		s.Set(label, value)
	}
	return found
}

func (s *SessionImpl) SaveToDB() {
	if s.ID == "" {
		return
	}
	for k, v := range s.values {
		dt := time.Now().Format(time.RFC3339)
		_, err := DB.Exec("INSERT into session_entries VALUES (?, ?, ?, ?) on duplicate key update value = ?, updated_at = ?", s.ID, k, v, dt, v, dt)
		if err != nil {
			log.Default().Printf("error deleting all session entries from db (session: %s): %s", s.ID, err.Error())
			return
		}
	}
}

func initSession() {
	var sessionID string
	cookies := make(map[string]string)
	cookie := os.Getenv(EnvCOOKIE)
	log.Default().Println("Cookie: ", cookie)

	if len(cookie) != 0 {
		pairs := strings.Split(cookie, ";")
		for _, p := range pairs {
			s := strings.TrimSpace(p)
			kv := strings.Split(s, "=")
			cookies[kv[0]] = kv[1]
		}
	}

	Session = &SessionImpl{
		values:  make(map[string]string),
		cookies: cookies,
	}
	if sid, found := cookies[SessionIDCookieName]; found {
		log.Default().Println("Found session cookie: ", sid)
		Session.ID = sid
		found := Session.LoadFromDB()
		if found {
			log.Default().Printf("Session Variables: %+v", Session.values)
			return
		}
	}

	sessionID = uuid.NewString()
	cookies[SessionIDCookieName] = sessionID
	Session.ID = sessionID
	Session.cookies = cookies
	Session.Set("created_at", time.Now().Format(time.RFC3339))
	log.Default().Printf("created new session: %+v", Session)
}
