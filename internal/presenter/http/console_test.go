package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
)

func consoleRequest(method, path string, form url.Values, authenticated bool) *http.Request {
	body := ""
	if form != nil {
		body = form.Encode()
	}
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if authenticated {
		request.AddCookie(&http.Cookie{Name: "gophermart_token", Value: "ok"})
	}
	return request
}

func TestConsolePage(t *testing.T) {
	server, _, _, _, _ := newTestServer()
	for _, test := range []struct {
		name       string
		path       string
		wantStatus int
		wantBody   []string
	}{
		{name: "root redirects", path: "/", wantStatus: http.StatusFound},
		{name: "console", path: "/ui", wantStatus: http.StatusOK, wantBody: []string{"htmx.org@2.0.4", `hx-post="/ui/call"`, "/api/user/register"}},
		{name: "swagger", path: "/swagger", wantStatus: http.StatusOK, wantBody: []string{"swagger-ui", `"/api/openapi.yaml"`}},
		{name: "swagger slash", path: "/swagger/", wantStatus: http.StatusOK, wantBody: []string{"swagger-ui"}},
		{name: "spec", path: "/api/openapi.yaml", wantStatus: http.StatusOK, wantBody: []string{"openapi: 3.0.3", "/api/user/balance/withdraw"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			for _, want := range test.wantBody {
				if !strings.Contains(recorder.Body.String(), want) {
					t.Fatalf("body does not contain %q", want)
				}
			}
		})
	}

	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if location := recorder.Header().Get("Location"); location != "/ui" {
		t.Fatalf("redirect location = %q", location)
	}
}

func TestConsoleCallReportsAPIStatus(t *testing.T) {
	server, _, orders, balance, _ := newTestServer()
	balance.withdrawErr = loyalty.ErrInsufficientFunds
	accrual := loyalty.Amount(50050)
	listed := []order.Order{{Number: "9278923470", Status: order.StatusProcessed, Accrual: &accrual, UploadedAt: time.Now()}}

	for _, test := range []struct {
		name          string
		form          url.Values
		authenticated bool
		wantBody      []string
		wantTrigger   bool
	}{
		{
			name:        "register succeeds",
			form:        url.Values{"op": {"register"}, "login": {"gopher"}, "password": {"secret"}},
			wantBody:    []string{"200 OK", "/api/user/register", "{&#34;login&#34;:&#34;gopher&#34;,&#34;password&#34;:&#34;secret&#34;}"},
			wantTrigger: true,
		},
		{
			name:     "list requires authentication",
			form:     url.Values{"op": {"list-orders"}},
			wantBody: []string{"401 Unauthorized", "/api/user/orders"},
		},
		{
			name:          "order accepted",
			form:          url.Values{"op": {"submit-order"}, "number": {" 9278923470 "}},
			authenticated: true,
			wantBody:      []string{"202 Accepted", "9278923470"},
			wantTrigger:   true,
		},
		{
			name:          "insufficient funds",
			form:          url.Values{"op": {"withdraw"}, "order": {"2377225624"}, "sum": {"751"}},
			authenticated: true,
			wantBody:      []string{"402 Payment Required", "{&#34;order&#34;:&#34;2377225624&#34;,&#34;sum&#34;:751}"},
			wantTrigger:   true,
		},
		{
			name:          "malformed sum reaches the handler",
			form:          url.Values{"op": {"withdraw"}, "order": {"2377225624"}, "sum": {"много"}},
			authenticated: true,
			wantBody:      []string{"400 Bad Request"},
			wantTrigger:   true,
		},
		{
			name:          "json body is pretty printed",
			form:          url.Values{"op": {"list-orders"}},
			authenticated: true,
			wantBody:      []string{"200 OK", "  {\n    &#34;number&#34;: &#34;9278923470&#34;", "&#34;accrual&#34;: 500.5"},
		},
		{
			name:          "empty list keeps its code",
			form:          url.Values{"op": {"list-withdrawals"}},
			authenticated: true,
			wantBody:      []string{"204 No Content", "Пустое"},
		},
		{
			name:     "unknown operation",
			form:     url.Values{"op": {"drop-database"}},
			wantBody: []string{"Неизвестная операция"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			orders.err = nil
			orders.items = listed
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, consoleRequest(http.MethodPost, "/ui/call", test.form, test.authenticated))
			if recorder.Code != http.StatusOK {
				t.Fatalf("fragment status = %d, want 200", recorder.Code)
			}
			for _, want := range test.wantBody {
				if !strings.Contains(recorder.Body.String(), want) {
					t.Fatalf("fragment does not contain %q:\n%s", want, recorder.Body.String())
				}
			}
			if trigger := recorder.Header().Get("HX-Trigger") == refreshEvent; trigger != test.wantTrigger {
				t.Fatalf("HX-Trigger = %q, want refresh=%t", recorder.Header().Get("HX-Trigger"), test.wantTrigger)
			}
		})
	}
}

func TestConsoleCallPropagatesToken(t *testing.T) {
	server, _, _, _, _ := newTestServer()
	recorder := httptest.NewRecorder()
	form := url.Values{"op": {"login"}, "login": {"gopher"}, "password": {"secret"}}
	server.Router().ServeHTTP(recorder, consoleRequest(http.MethodPost, "/ui/call", form, false))

	response := recorder.Result()
	defer response.Body.Close()
	cookies := response.Cookies()
	if len(cookies) != 1 || cookies[0].Name != "gophermart_token" || cookies[0].Value != "logged" {
		t.Fatalf("cookie was not propagated to the browser: %+v", cookies)
	}
}

func TestConsoleViews(t *testing.T) {
	server, _, orders, balance, _ := newTestServer()
	accrual := loyalty.Amount(50050)
	orders.items = []order.Order{
		{Number: "9278923470", Status: order.StatusProcessed, Accrual: &accrual, UploadedAt: time.Now()},
		{Number: "12345678903", Status: order.StatusNew, UploadedAt: time.Now()},
	}
	balance.account = loyalty.Account{Current: 50050, Withdrawn: 2500}
	balance.withdrawals = []loyalty.Withdrawal{{Order: "2377225624", Amount: 2500, ProcessedAt: time.Now()}}

	for _, test := range []struct {
		name          string
		path          string
		authenticated bool
		wantBody      []string
	}{
		{name: "orders table", path: "/ui/orders", authenticated: true, wantBody: []string{"9278923470", "PROCESSED", "500.5", "12345678903", "NEW"}},
		{name: "balance metrics", path: "/ui/balance", authenticated: true, wantBody: []string{"500.5", "25"}},
		{name: "withdrawals table", path: "/ui/withdrawals", authenticated: true, wantBody: []string{"2377225624", "25"}},
		{name: "orders unauthenticated", path: "/ui/orders", wantBody: []string{"401"}},
		{name: "balance unauthenticated", path: "/ui/balance", wantBody: []string{"401"}},
		{name: "withdrawals unauthenticated", path: "/ui/withdrawals", wantBody: []string{"401"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, consoleRequest(http.MethodGet, test.path, nil, test.authenticated))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			for _, want := range test.wantBody {
				if !strings.Contains(recorder.Body.String(), want) {
					t.Fatalf("fragment does not contain %q:\n%s", want, recorder.Body.String())
				}
			}
		})
	}

	orders.items = nil
	balance.withdrawals = nil
	for _, path := range []string{"/ui/orders", "/ui/withdrawals"} {
		recorder := httptest.NewRecorder()
		server.Router().ServeHTTP(recorder, consoleRequest(http.MethodGet, path, nil, true))
		if !strings.Contains(recorder.Body.String(), "204 No Content") {
			t.Fatalf("%s empty state:\n%s", path, recorder.Body.String())
		}
	}
}

func TestStatusClass(t *testing.T) {
	for status, want := range map[int]string{204: "ok", 302: "muted", 409: "warn", 500: "err"} {
		if got := statusClass(status); got != want {
			t.Fatalf("statusClass(%d) = %q, want %q", status, got, want)
		}
	}
	for status, want := range map[order.Status]string{
		order.StatusProcessed: "ok", order.StatusInvalid: "err", order.StatusNew: "muted",
	} {
		if got := orderClass(status); got != want {
			t.Fatalf("orderClass(%s) = %q, want %q", status, got, want)
		}
	}
}
