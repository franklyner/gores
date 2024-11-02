package middleware

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

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

func init() {
	DefaultRouter = &Router{
		handlers: make(map[string]func(Request, *Response)),
	}
	DefaultRouter.AddHandler(HandlerNotFoud, func(req Request, resp *Response) {
		fmt.Fprintln(resp.Body, "No handler defined for this path!")
	})

	f, err := os.OpenFile("gores.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	log.Default().SetOutput(f)
}

type Router struct {
	handlers map[string]func(Request, *Response)
}

func (r *Router) AddHandler(path string, handler func(Request, *Response)) {
	r.handlers[path] = handler
}

func (r *Router) Handle() {
	req := createRequest()
	resp := createResponse()
	initSession()
	handler, found := r.handlers[req.Path]
	if found {
		handler(req, resp)
	} else {
		r.handlers[HandlerNotFoud](req, resp)
	}

	fmt.Printf("Set-Cookie: %s\n", Session.GetCoockieStr())
	for k, v := range resp.Headers {
		fmt.Printf("%s: %s\n", k, v)
	}
	fmt.Println()
	fmt.Print(resp.Body)
}

type Request struct {
	Path  string
	Query map[string]string
}

type Response struct {
	Headers map[string]string
	Body    *bytes.Buffer
}

func createRequest() Request {
	qry := make(map[string]string)
	qryStr := os.Getenv(EnvQUERY)
	if len(qryStr) != 0 {
		tuples := strings.Split(qryStr, "&")
		for _, t := range tuples {
			kv := strings.Split(t, "=")
			qry[kv[0]] = kv[1]
		}
	}
	return Request{
		Path:  os.Getenv(EnvPATH),
		Query: qry,
	}
}

func createResponse() *Response {
	resp := &Response{
		Headers: make(map[string]string),
		Body:    &bytes.Buffer{},
	}

	resp.Headers["Content-Type"] = "text/html"

	return resp
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

	if sid, found := cookies[SessionIDCookieName]; found {
		sessionID = sid
	} else {
		sessionID = uuid.NewString()
		cookies[SessionIDCookieName] = sessionID
	}
	Session = &SessionImpl{
		ID:      sessionID,
		values:  make(map[string]string),
		cookies: cookies,
	}
}
