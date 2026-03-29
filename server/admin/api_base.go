package admin

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/cherts/anylink/base"
	"github.com/cherts/anylink/pkg/utils"
	"github.com/gorilla/mux"
	"github.com/xlzd/gotp"
)

// Login login interface
func Login(w http.ResponseWriter, r *http.Request) {
	// TODO Debug information output
	// hd, _ := httputil.DumpRequest(r, true)
	// fmt.Println("DumpRequest: ", string(hd))

	_ = r.ParseForm()
	adminUser := r.PostFormValue("admin_user")
	adminPass := r.PostFormValue("admin_pass")

	// Enable OTP verification
	if base.Cfg.AdminOtp != "" {
		pwd := adminPass
		pl := len(pwd)
		if pl < 6 {
			RespError(w, RespUserOrPassErr)
			base.Error(adminUser, "Admin OTP error")
			return
		}
		// Determine OTP information
		adminPass = pwd[:pl-6]
		otp := pwd[pl-6:]

		totp := gotp.NewDefaultTOTP(base.Cfg.AdminOtp)
		unix := time.Now().Unix()
		verify := totp.Verify(otp, unix)

		if !verify {
			RespError(w, RespUserOrPassErr)
			base.Error(adminUser, "Admin OTP error")
			return
		}
	}

	// Authentication error
	if !(adminUser == base.Cfg.AdminUser &&
		utils.PasswordVerify(adminPass, base.Cfg.AdminPass)) {
		RespError(w, RespUserOrPassErr)
		base.Error(adminUser, "Wrong username or password")
		return
	}

	// token validity period
	expiresAt := time.Now().Unix() + 3600*3
	jwtData := map[string]interface{}{"admin_user": adminUser}
	tokenString, err := SetJwtData(jwtData, expiresAt)
	if err != nil {
		RespError(w, 1, err)
		return
	}

	data := make(map[string]interface{})
	data["token"] = tokenString
	data["admin_user"] = adminUser
	data["expires_at"] = expiresAt

	ck := &http.Cookie{
		Name:     "jwt",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(w, ck)

	RespSucess(w, data)
}

func authMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == http.MethodOptions {
			// w.WriteHeader(http.StatusOK)
			// The official environment does not support OPTIONS
			w.WriteHeader(http.StatusForbidden)
			return
		}

		route := mux.CurrentRoute(r)
		name := route.GetName()
		// fmt.Println("bb", r.URL.Path, name)
		if utils.InArrStr([]string{"login", "index", "static"}, name) {
			// No authentication
			next.ServeHTTP(w, r)
			return
		}

		// Perform login authentication
		jwtToken := r.Header.Get("Jwt")
		if jwtToken == "" {
			jwtToken = r.FormValue("jwt")
		}
		if jwtToken == "" {
			cc, err := r.Cookie("jwt")
			if err == nil {
				jwtToken = cc.Value
			}
		}
		data, err := GetJwtData(jwtToken)
		if err != nil || base.Cfg.AdminUser != fmt.Sprint(data["admin_user"]) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func recoverHttp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				base.Error(err, string(stack))
				// http.Error(w, "Internal Server Error", 500)
				RespError(w, 500, "Internal Server Error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}