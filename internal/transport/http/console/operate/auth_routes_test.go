package operate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
)

func TestAuthRoutesLoginTokenAndLogout(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &authdomain.AuthService{Store: authdomain.NewMemorySessionStore()}
	RegisterAuthRoutes(router, service)

	req := httptest.NewRequest(http.MethodPost, "/operate/auth/login", bytes.NewReader([]byte(`{"username":"admin","password":"secret"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status got %d body %s", rec.Code, rec.Body.String())
	}
	var loginResult struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&loginResult); err != nil {
		t.Fatal(err)
	}
	if loginResult.Data.Token == "" {
		t.Fatalf("expected token")
	}

	tokenReq := httptest.NewRequest(http.MethodGet, "/operate/auth/token?token="+loginResult.Data.Token, nil)
	tokenRec := httptest.NewRecorder()
	router.ServeHTTP(tokenRec, tokenReq)
	if tokenRec.Code != http.StatusOK {
		t.Fatalf("token status got %d body %s", tokenRec.Code, tokenRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/operate/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginResult.Data.Token)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status got %d body %s", logoutRec.Code, logoutRec.Body.String())
	}
}

func TestMerchantAuthRoutesLogin(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	service := &authdomain.AuthService{Store: authdomain.NewMemorySessionStore()}
	RegisterAuthRoutes(router, service)

	req := httptest.NewRequest(http.MethodPost, "/merchant/auth/login", bytes.NewReader([]byte(`{"username":"merchant","password":"secret","merchantId":"1","userId":"2"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status got %d body %s", rec.Code, rec.Body.String())
	}
	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected login result: %+v", result)
	}
}
