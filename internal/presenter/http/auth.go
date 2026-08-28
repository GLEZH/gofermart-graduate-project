package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
	usecaseauth "github.com/GLEZH/gofermart-graduate-project/internal/usecase/auth"
)

const maxJSONBody = 1 << 20

type credentialsRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	credentials, err := decodeCredentials(w, r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	token, err := s.auth.Register(r.Context(), credentials)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidInput):
			w.WriteHeader(http.StatusBadRequest)
		case errors.Is(err, user.ErrLoginTaken):
			w.WriteHeader(http.StatusConflict)
		default:
			s.internalError(w, err)
		}
		return
	}
	s.writeToken(w, token)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	credentials, err := decodeCredentials(w, r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	token, err := s.auth.Login(r.Context(), credentials)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidInput):
			w.WriteHeader(http.StatusBadRequest)
		case errors.Is(err, user.ErrInvalidCredentials):
			w.WriteHeader(http.StatusUnauthorized)
		default:
			s.internalError(w, err)
		}
		return
	}
	s.writeToken(w, token)
	w.WriteHeader(http.StatusOK)
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (usecaseauth.Credentials, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	var request credentialsRequest
	if err := decoder.Decode(&request); err != nil {
		return usecaseauth.Credentials{}, err
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return usecaseauth.Credentials{}, err
	}
	return usecaseauth.Credentials{Login: request.Login, Password: request.Password}, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func (s *Server) writeToken(w http.ResponseWriter, token string) {
	w.Header().Set("Authorization", "Bearer "+token)
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		MaxAge:   int((24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if value := r.Header.Get("Authorization"); strings.HasPrefix(value, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
		}
		if token == "" {
			if cookie, err := r.Cookie(s.cookie); err == nil {
				token = cookie.Value
			}
		}
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		userID, err := s.tokens.Verify(token)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), userID)))
	})
}
