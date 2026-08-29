package httpapi

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templateFS embed.FS

var pages = template.Must(template.New("gophermart").Funcs(template.FuncMap{
	"statusClass": statusClass,
	"orderClass":  orderClass,
}).ParseFS(templateFS, "templates/*.html"))

const (
	maxConsoleForm = 64 << 10
	refreshEvent   = "gophermart-refresh"
)

// consoleOperation описывает один вызов API. Консоль умеет вызывать только
// операции из этого списка: путь и метод задаёт сервер, браузер выбирает лишь ключ.
type consoleOperation struct {
	Title  string
	Method string
	Path   string
	Type   string
	Body   func(form url.Values) string
	Mutate bool
}

var consoleOperations = map[string]consoleOperation{
	"register": {
		Title: "Регистрация", Method: http.MethodPost, Path: "/api/user/register",
		Type: "application/json", Body: credentialsBody, Mutate: true,
	},
	"login": {
		Title: "Аутентификация", Method: http.MethodPost, Path: "/api/user/login",
		Type: "application/json", Body: credentialsBody, Mutate: true,
	},
	"submit-order": {
		Title: "Загрузка номера заказа", Method: http.MethodPost, Path: "/api/user/orders",
		Type: "text/plain", Body: func(form url.Values) string { return strings.TrimSpace(form.Get("number")) }, Mutate: true,
	},
	"list-orders": {
		Title: "Список заказов", Method: http.MethodGet, Path: "/api/user/orders",
	},
	"balance": {
		Title: "Баланс", Method: http.MethodGet, Path: "/api/user/balance",
	},
	"withdraw": {
		Title: "Списание баллов", Method: http.MethodPost, Path: "/api/user/balance/withdraw",
		Type: "application/json", Body: withdrawalBody, Mutate: true,
	},
	"list-withdrawals": {
		Title: "Списания", Method: http.MethodGet, Path: "/api/user/withdrawals",
	},
}

type consoleResult struct {
	Title   string
	Method  string
	Path    string
	Request string
	Status  int
	Reason  string
	Body    string
	Empty   bool
}

func (s *Server) mountConsole(router chi.Router) {
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui", http.StatusFound)
	})
	router.Get("/ui", s.consolePage)
	router.Post("/ui/call", s.consoleCall)
	router.Get("/ui/orders", s.consoleOrders)
	router.Get("/ui/balance", s.consoleBalance)
	router.Get("/ui/withdrawals", s.consoleWithdrawals)
}

func (s *Server) consolePage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "console.html", map[string]string{"Spec": specPath, "Swagger": swaggerPath})
}

func (s *Server) consoleCall(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxConsoleForm)
	if err := r.ParseForm(); err != nil {
		s.render(w, "result.html", consoleResult{Title: "Некорректная форма", Reason: err.Error()})
		return
	}
	operation, ok := consoleOperations[r.PostFormValue("op")]
	if !ok {
		s.render(w, "result.html", consoleResult{Title: "Неизвестная операция"})
		return
	}

	body := ""
	if operation.Body != nil {
		body = operation.Body(r.PostForm)
	}
	status, _, payload := s.callAPI(w, r, operation, body)
	if operation.Mutate {
		w.Header().Set("HX-Trigger", refreshEvent)
	}
	s.render(w, "result.html", consoleResult{
		Title:   operation.Title,
		Method:  operation.Method,
		Path:    operation.Path,
		Request: body,
		Status:  status,
		Reason:  http.StatusText(status),
		Body:    prettyJSON(payload),
		Empty:   len(bytes.TrimSpace(payload)) == 0,
	})
}

func (s *Server) consoleOrders(w http.ResponseWriter, r *http.Request) {
	status, _, payload := s.callAPI(w, r, consoleOperations["list-orders"], "")
	view := struct {
		Status int
		Items  []orderResponse
	}{Status: status}
	if status == http.StatusOK {
		_ = json.Unmarshal(payload, &view.Items)
	}
	s.render(w, "orders.html", view)
}

func (s *Server) consoleBalance(w http.ResponseWriter, r *http.Request) {
	status, _, payload := s.callAPI(w, r, consoleOperations["balance"], "")
	view := struct {
		Status  int
		Balance balanceResponse
	}{Status: status}
	if status == http.StatusOK {
		_ = json.Unmarshal(payload, &view.Balance)
	}
	s.render(w, "balance.html", view)
}

func (s *Server) consoleWithdrawals(w http.ResponseWriter, r *http.Request) {
	status, _, payload := s.callAPI(w, r, consoleOperations["list-withdrawals"], "")
	view := struct {
		Status int
		Items  []withdrawalResponse
	}{Status: status}
	if status == http.StatusOK {
		_ = json.Unmarshal(payload, &view.Items)
	}
	s.render(w, "withdrawals.html", view)
}

// callAPI выполняет операцию через тот же роутер, что обслуживает внешних клиентов,
// поэтому консоль показывает настоящие коды ответов, а не их пересказ. Токен
// аутентификации переносится в обе стороны: из запроса браузера во внутренний вызов
// и обратно, чтобы вход через консоль заводил сессию в браузере.
func (s *Server) callAPI(w http.ResponseWriter, r *http.Request, operation consoleOperation, body string) (int, http.Header, []byte) {
	// Контекст маршрутизации обязательно свежий: chi переиспользовал бы разбор
	// внешнего запроса и отвечал бы 405 на вложенный вызов с другим методом.
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, chi.NewRouteContext())
	request := (&http.Request{
		Method:        operation.Method,
		URL:           &url.URL{Path: operation.Path},
		RequestURI:    operation.Path,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Host:          r.Host,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}).WithContext(ctx)
	if operation.Type != "" {
		request.Header.Set("Content-Type", operation.Type)
	}
	if value := r.Header.Get("Authorization"); value != "" {
		request.Header.Set("Authorization", value)
	}
	if cookie, err := r.Cookie(s.cookie); err == nil {
		request.AddCookie(cookie)
	}

	captured := &capturedResponse{header: make(http.Header)}
	s.api.ServeHTTP(captured, request)

	for _, cookie := range captured.header.Values("Set-Cookie") {
		w.Header().Add("Set-Cookie", cookie)
	}
	return captured.statusCode(), captured.header, captured.body.Bytes()
}

// capturedResponse собирает ответ внутреннего вызова вместо отправки его клиенту.
type capturedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *capturedResponse) Header() http.Header { return c.header }

func (c *capturedResponse) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}

func (c *capturedResponse) Write(data []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	return c.body.Write(data)
}

func (c *capturedResponse) statusCode() int {
	if c.status == 0 {
		return http.StatusOK
	}
	return c.status
}

// render всегда отвечает 200: код вызванного хендлера показывается внутри
// фрагмента, поэтому htmx подставляет разметку без настройки обработки ошибок.
// Шаблон рендерится в буфер, чтобы сбой разметки не оставил клиенту полуответ.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	var buffer bytes.Buffer
	if err := pages.ExecuteTemplate(&buffer, name, data); err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := buffer.WriteTo(w); err != nil && s.log != nil {
		s.log.Errorw("write html", "template", name, "error", err)
	}
}

func credentialsBody(form url.Values) string {
	body, err := json.Marshal(map[string]string{
		"login":    strings.TrimSpace(form.Get("login")),
		"password": form.Get("password"),
	})
	if err != nil {
		return ""
	}
	return string(body)
}

// withdrawalBody подставляет сумму без разбора: так консоль умеет отправить и
// заведомо некорректное тело, чтобы показать ответ 400.
func withdrawalBody(form url.Values) string {
	number, err := json.Marshal(strings.TrimSpace(form.Get("order")))
	if err != nil {
		return ""
	}
	sum := strings.TrimSpace(form.Get("sum"))
	if sum == "" {
		sum = "null"
	}
	return `{"order":` + string(number) + `,"sum":` + sum + `}`
}

func prettyJSON(payload []byte) string {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return ""
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, trimmed, "", "  "); err != nil {
		return string(trimmed)
	}
	return indented.String()
}

func orderClass(status order.Status) string {
	switch status {
	case order.StatusProcessed:
		return "ok"
	case order.StatusInvalid:
		return "err"
	case order.StatusProcessing:
		return ""
	default:
		return "muted"
	}
}

func statusClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "ok"
	case status >= 400 && status < 500:
		return "warn"
	case status >= 500:
		return "err"
	default:
		return "muted"
	}
}
