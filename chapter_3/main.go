package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/stretchr/gomniauth"
	"github.com/stretchr/gomniauth/providers/facebook"
	"github.com/stretchr/gomniauth/providers/github"
	"github.com/stretchr/gomniauth/providers/google"
	"github.com/stretchr/objx"
)

var avatars Avatar = TryAvatars{
	UseFileSystemAvatar,
	UseAuthAvatar,
	UseGravatar,
}

// templは1つのテンプレートを表します
type templateHandler struct {
	once     sync.Once
	filename string
	templ    *template.Template
}

// ServerHTTPはHTTPリクエストを処理します
func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.once.Do(func() {
		t.templ = template.Must(template.ParseFiles(filepath.Join("templates", t.filename)))
	})
	data := map[string]interface{}{
		"Host": r.Host,
	}
	if authCockie, err := r.Cookie("auth"); err == nil {
		data["UserData"] = objx.MustFromBase64(authCockie.Value)
	}
	t.templ.Execute(w, data)
}

func main() {
	var addr = flag.String("host", ":8080", "アプリケーションのアドレス")
	flag.Parse()

	// Gomniauthのセットアップ
	gomniauth.SetSecurityKey("セキュリティーキー")
	gomniauth.WithProviders(
		facebook.New("クライアントID", "秘密の値", "http://localhost:8080/auth/callback/facebook"),
		github.New("クライアントID", "秘密の値", "http://localhost:8080/auth/callback/github"),
		google.New("クライアントID", "秘密の値", "http://localhost:8080/auth/callback/google"),
	)

	r := newRoom(UseFileSystemAvatar)
	//r.tracer = trace.New(os.Stdout)

	// URI設定
	http.Handle("/", &templateHandler{filename: "chat.html"})
	http.Handle("/chat", MustAuth(&templateHandler{filename: "chat.html"}))
	http.Handle("/login", &templateHandler{filename: "login.html"})
	http.Handle("/upload", &templateHandler{filename: "upload.html"})
	http.Handle("/room", r)

	// リソースURIの登録
	// assetsディレクトリ登録設定
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets/"))))
	// アバター画像のホスティングディレクトリを登録
	http.Handle("/avatars/", http.StripPrefix("/avatars/", http.FileServer(http.Dir("./avatars"))))

	// ハンドラ関数の登録
	// 認証
	http.HandleFunc("/auth/", loginHandler)
	// ログアウト
	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:   "auth",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		w.Header()["Location"] = []string{"/chat"}
		w.WriteHeader(http.StatusTemporaryRedirect)
	})
	// アップローダー
	http.HandleFunc("/uploader", uploaderHandler)

	// チャットルームの開始
	go r.run()

	log.Println("Webサーバを開始します。ポート:", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServer", err)
	}
}
