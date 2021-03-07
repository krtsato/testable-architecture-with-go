package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.dena.jp/swet/go-sampleapi/internal/repository"
	"github.dena.jp/swet/go-sampleapi/internal/usecase"

	"github.dena.jp/swet/go-sampleapi/internal/apierr"

	"github.com/jmoiron/sqlx"

	_ "github.com/go-sql-driver/mysql"

	"github.dena.jp/swet/go-sampleapi/internal/config"
	"github.dena.jp/swet/go-sampleapi/internal/handler"
	"github.dena.jp/swet/go-sampleapi/internal/logging"
)

// testで作成するuserのdata
const (
	email    = "test@example.com"
	password = "passw0rd!123"
)

// POST /users に対するtest
// 正常なパラメータでリクエストを行う
func Test_E2E_PostUser(t *testing.T) {
	db := sqlx.MustConnect("mysql", config.Config().DBSrc())
	defer func() {
		// DBのcleanを行う
		db.MustExec("set foreign_key_checks = 0")
		db.MustExec("truncate table users")
		db.MustExec("set foreign_key_checks = 1")
		db.Close()
	}()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(&handler.ReqPostUserJSON{
		FirstName: "テスト姓",
		LastName:  "テスト名",
		Email:     email,
		Password:  password,
	}); err != nil {
		t.Fatal(err)
	}

	// requestをシュミレートする
	req := httptest.NewRequest(http.MethodPost, "/", &body)
	rec := httptest.NewRecorder()

	userUsecase := usecase.NewUser(repository.NewUser(), db)
	handler.PostUser(userUsecase, logging.Logger()).ServeHTTP(rec, req)

	// responseのStatus Codeをチェックする
	if rec.Code != http.StatusCreated {
		t.Errorf("status code must be 201 but: %d", rec.Code)
		t.Fatalf("body: %s", rec.Body.String())
	}

	var result handler.ResPostUserJSON
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	// responseで返ってきたIDでuserが作られているかどうかをチェックする
	var actual string
	if err := db.Get(&actual, "select email from users where id = ?", result.ID); err != nil {
		t.Fatal(err)
	}
	if actual != email {
		t.Errorf("email must be %s but %s", email, actual)
	}
}

// POST /users に対するtest
// 重複するuserでリクエストを行う
func Test_E2E_PostUser_DuplicateEmail(t *testing.T) {
	db := sqlx.MustConnect("mysql", config.Config().DBSrc())
	defer func() {
		db.MustExec("set foreign_key_checks = 0")
		db.MustExec("truncate table users")
		db.MustExec("set foreign_key_checks = 1")
		db.Close()
	}()

	email := "test@example.com"

	if _, err := db.Exec("insert into users(first_name, last_name, email, password_hash) values (?, ?, ?, ?)", "dummy_first_name", "dummy_last_name", email, "dummy_password"); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(&handler.ReqPostUserJSON{
		FirstName: "テスト姓",
		LastName:  "テスト名",
		Email:     email,
		Password:  "passw0rd1234",
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	rec := httptest.NewRecorder()

	userUsecase := usecase.NewUser(repository.NewUser(), db)
	handler.PostUser(userUsecase, logging.Logger()).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code must be 400 but: %d", rec.Code)
	}

	var result handler.ResError
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	// responseのMessageをチェックする
	if result.Message != string(apierr.ErrEmailAlreadyExists) {
		t.Errorf("error Message must be %s but %s", apierr.ErrEmailAlreadyExists, result.Message)
	}
}

// passwordでvalidation errorのケース
func Test_E2E_PostUser_ValidationError_Short_Password(t *testing.T) {
	db := sqlx.MustConnect("mysql", config.Config().DBSrc())
	defer func() {
		db.MustExec("set foreign_key_checks = 0")
		db.MustExec("truncate table users")
		db.MustExec("set foreign_key_checks = 1")
		db.Close()
	}()

	var body bytes.Buffer

	// 他形式のpasswordを使ってテストしたいときテーブルテストを実施する
	// 参考：internal/validator/validation_test.go
	if err := json.NewEncoder(&body).Encode(&handler.ReqPostUserJSON{
		FirstName: "name",
		LastName:  "name",
		Email:     "aaa@example.com",
		Password:  "passw0rd", // 12文字より短い
	}); err != nil {
		t.Fatal(err)
	}

	// requestをシュミレートする
	req := httptest.NewRequest("POST", "/", &body)
	rec := httptest.NewRecorder()

	uuc := usecase.NewUser(repository.NewUser(), db)
	handler.PostUser(uuc, logging.Logger()).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code must be 400 but: %d", rec.Code)
	}

	var result handler.ResError

	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if result.Message != string(apierr.ErrBadRequest) {
		t.Errorf("error Message must be %s but %s", apierr.ErrBadRequest, result.Message)
	}
}
