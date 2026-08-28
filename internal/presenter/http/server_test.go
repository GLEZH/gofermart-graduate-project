package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
	usecaseauth "github.com/GLEZH/gofermart-graduate-project/internal/usecase/auth"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type fakeAuth struct {
	registerToken string
	loginToken    string
	registerErr   error
	loginErr      error
}

func (f *fakeAuth) Register(context.Context, usecaseauth.Credentials) (string, error) {
	return f.registerToken, f.registerErr
}
func (f *fakeAuth) Login(context.Context, usecaseauth.Credentials) (string, error) {
	return f.loginToken, f.loginErr
}

type fakeOrders struct {
	items []order.Order
	err   error
	panic bool
}

func (f *fakeOrders) Submit(context.Context, user.ID, string) error { return f.err }
func (f *fakeOrders) List(context.Context, user.ID) ([]order.Order, error) {
	if f.panic {
		panic("test panic")
	}
	return f.items, f.err
}

type fakeBalance struct {
	account     loyalty.Account
	withdrawals []loyalty.Withdrawal
	getErr      error
	withdrawErr error
	listErr     error
}

func (f *fakeBalance) Get(context.Context, user.ID) (loyalty.Account, error) {
	return f.account, f.getErr
}
func (f *fakeBalance) Withdraw(context.Context, user.ID, string, loyalty.Amount) error {
	return f.withdrawErr
}
func (f *fakeBalance) Withdrawals(context.Context, user.ID) ([]loyalty.Withdrawal, error) {
	return f.withdrawals, f.listErr
}

type fakeTokens struct{}

func (fakeTokens) Verify(value string) (user.ID, error) {
	if value != "ok" {
		return 0, errors.New("invalid token")
	}
	return 7, nil
}

func newTestServer() (*Server, *fakeAuth, *fakeOrders, *fakeBalance, *observer.ObservedLogs) {
	auth := &fakeAuth{registerToken: "registered", loginToken: "logged"}
	orders := &fakeOrders{}
	balance := &fakeBalance{}
	core, logs := observer.New(zapcore.DebugLevel)
	server := NewServer(auth, orders, balance, fakeTokens{}, zap.New(core).Sugar())
	return server, auth, orders, balance, logs
}

func TestAuthHandlers(t *testing.T) {
	server, auth, _, _, _ := newTestServer()
	tests := []struct {
		name       string
		path       string
		body       string
		setup      func()
		wantStatus int
		wantToken  string
	}{
		{name: "register", path: "/api/user/register", body: `{"login":"user","password":"pass"}`, wantStatus: 200, wantToken: "registered"},
		{name: "register malformed", path: "/api/user/register", body: `{`, wantStatus: 400},
		{name: "register duplicate json", path: "/api/user/register", body: `{} {}`, wantStatus: 400},
		{name: "register invalid", path: "/api/user/register", body: `{}`, setup: func() { auth.registerErr = user.ErrInvalidInput }, wantStatus: 400},
		{name: "register conflict", path: "/api/user/register", body: `{}`, setup: func() { auth.registerErr = user.ErrLoginTaken }, wantStatus: 409},
		{name: "register internal", path: "/api/user/register", body: `{}`, setup: func() { auth.registerErr = errors.New("db") }, wantStatus: 500},
		{name: "login", path: "/api/user/login", body: `{"login":"user","password":"pass"}`, wantStatus: 200, wantToken: "logged"},
		{name: "login malformed", path: "/api/user/login", body: `{`, wantStatus: 400},
		{name: "login invalid input", path: "/api/user/login", body: `{}`, setup: func() { auth.loginErr = user.ErrInvalidInput }, wantStatus: 400},
		{name: "login unauthorized", path: "/api/user/login", body: `{}`, setup: func() { auth.loginErr = user.ErrInvalidCredentials }, wantStatus: 401},
		{name: "login internal", path: "/api/user/login", body: `{}`, setup: func() { auth.loginErr = errors.New("db") }, wantStatus: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			auth.registerErr = nil
			auth.loginErr = nil
			if test.setup != nil {
				test.setup()
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if test.wantToken != "" {
				if recorder.Header().Get("Authorization") != "Bearer "+test.wantToken || len(recorder.Result().Cookies()) != 1 {
					t.Fatalf("authentication response = %v", recorder.Header())
				}
			}
		})
	}
}

func TestAuthentication(t *testing.T) {
	server, _, _, balance, _ := newTestServer()
	balance.account = loyalty.Account{Current: 100}
	for _, test := range []struct {
		name       string
		header     string
		cookie     *http.Cookie
		wantStatus int
	}{
		{name: "missing", wantStatus: 401},
		{name: "invalid bearer", header: "Bearer bad", wantStatus: 401},
		{name: "bearer", header: "Bearer ok", wantStatus: 200},
		{name: "cookie", cookie: &http.Cookie{Name: "gophermart_token", Value: "ok"}, wantStatus: 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
			request.Header.Set("Authorization", test.header)
			if test.cookie != nil {
				request.AddCookie(test.cookie)
			}
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}
}

func TestOrderHandlers(t *testing.T) {
	server, _, orders, _, _ := newTestServer()
	authRequest := func(method, path, body string) *http.Request {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer ok")
		return request
	}
	tests := []struct {
		name       string
		body       string
		err        error
		wantStatus int
	}{
		{name: "accepted", body: "9278923470", wantStatus: 202},
		{name: "same user", body: "9278923470", err: order.ErrOwnedByUser, wantStatus: 200},
		{name: "other user", body: "9278923470", err: order.ErrOwnedByAnotherUser, wantStatus: 409},
		{name: "bad checksum", body: "123", err: order.ErrInvalidNumber, wantStatus: 422},
		{name: "not digits", body: "abc", wantStatus: 400},
		{name: "empty", body: " ", wantStatus: 400},
		{name: "internal", body: "9278923470", err: errors.New("db"), wantStatus: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			orders.err = test.err
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, authRequest(http.MethodPost, "/api/user/orders", test.body))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}

	orders.err = nil
	orders.items = nil
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, authRequest(http.MethodGet, "/api/user/orders", ""))
	if recorder.Code != 204 {
		t.Fatalf("empty list status = %d", recorder.Code)
	}
	amount := loyalty.Amount(50050)
	orders.items = []order.Order{{Number: "9278923470", Status: order.StatusProcessed, Accrual: &amount, UploadedAt: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)}}
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, authRequest(http.MethodGet, "/api/user/orders", ""))
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"accrual":500.5`) || !strings.Contains(recorder.Body.String(), `"uploaded_at":"2026-08-28T12:00:00Z"`) {
		t.Fatalf("order response = %d %s", recorder.Code, recorder.Body.String())
	}
	orders.err = errors.New("db")
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, authRequest(http.MethodGet, "/api/user/orders", ""))
	if recorder.Code != 500 {
		t.Fatalf("internal list status = %d", recorder.Code)
	}
}

func TestBalanceHandlers(t *testing.T) {
	server, _, _, balance, _ := newTestServer()
	request := func(method, path, body string) *http.Request {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer ok")
		return r
	}
	balance.account = loyalty.Account{Current: 50050, Withdrawn: 4200}
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request(http.MethodGet, "/api/user/balance", ""))
	if recorder.Code != 200 || strings.TrimSpace(recorder.Body.String()) != `{"current":500.5,"withdrawn":42}` {
		t.Fatalf("balance response = %d %s", recorder.Code, recorder.Body.String())
	}
	balance.getErr = errors.New("db")
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request(http.MethodGet, "/api/user/balance", ""))
	if recorder.Code != 500 {
		t.Fatalf("balance error status = %d", recorder.Code)
	}
	balance.getErr = nil

	tests := []struct {
		name       string
		body       string
		err        error
		wantStatus int
	}{
		{name: "success", body: `{"order":"9278923470","sum":10}`, wantStatus: 200},
		{name: "malformed", body: `{`, wantStatus: 400},
		{name: "extra json", body: `{} {}`, wantStatus: 400},
		{name: "insufficient", body: `{"order":"9278923470","sum":10}`, err: loyalty.ErrInsufficientFunds, wantStatus: 402},
		{name: "invalid order", body: `{"order":"123","sum":10}`, err: order.ErrInvalidNumber, wantStatus: 422},
		{name: "invalid amount", body: `{"order":"9278923470","sum":0}`, err: loyalty.ErrInvalidAmount, wantStatus: 400},
		{name: "invalid amount json", body: `{"order":"9278923470","sum":-1}`, wantStatus: 400},
		{name: "internal", body: `{"order":"9278923470","sum":10}`, err: errors.New("db"), wantStatus: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			balance.withdrawErr = test.err
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, request(http.MethodPost, "/api/user/balance/withdraw", test.body))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}

	balance.withdrawals = nil
	balance.listErr = nil
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request(http.MethodGet, "/api/user/withdrawals", ""))
	if recorder.Code != 204 {
		t.Fatalf("empty withdrawals status = %d", recorder.Code)
	}
	balance.withdrawals = []loyalty.Withdrawal{{Order: "9278923470", Amount: 50000, ProcessedAt: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)}}
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request(http.MethodGet, "/api/user/withdrawals", ""))
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"sum":500`) {
		t.Fatalf("withdrawals response = %d %s", recorder.Code, recorder.Body.String())
	}
	balance.listErr = errors.New("db")
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request(http.MethodGet, "/api/user/withdrawals", ""))
	if recorder.Code != 500 {
		t.Fatalf("withdrawals error status = %d", recorder.Code)
	}
}

func TestGzipAndRecovery(t *testing.T) {
	server, _, orders, balance, logs := newTestServer()
	balance.account = loyalty.Account{Current: 100}
	request := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	request.Header.Set("Authorization", "Bearer ok")
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("response was not compressed")
	}
	reader, err := gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(reader)
	if !bytes.Contains(data, []byte(`"current":1`)) {
		t.Fatalf("gzip body = %s", data)
	}

	var body bytes.Buffer
	writer := gzip.NewWriter(&body)
	_, _ = writer.Write([]byte(`{"login":"user","password":"pass"}`))
	_ = writer.Close()
	request = httptest.NewRequest(http.MethodPost, "/api/user/register", &body)
	request.Header.Set("Content-Encoding", "gzip")
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("gzip request status = %d", recorder.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader("bad"))
	request.Header.Set("Content-Encoding", "gzip")
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != 400 {
		t.Fatalf("invalid gzip status = %d", recorder.Code)
	}

	orders.panic = true
	request = httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	request.Header.Set("Authorization", "Bearer ok")
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != 500 || logs.Len() == 0 {
		t.Fatalf("recovery status=%d logs=%d", recorder.Code, logs.Len())
	}
}

func TestWriteJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, http.StatusCreated, map[string]string{"ok": "yes"})
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body["ok"] != "yes" {
		t.Fatalf("response = %s, %v", recorder.Body.String(), err)
	}
}
