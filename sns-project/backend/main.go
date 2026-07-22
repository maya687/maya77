package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
}

type Post struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	Username   string    `json:"username"`
	AvatarURL  string    `json:"avatar_url"`
	Content    string    `json:"content"`
	ImageURL   string    `json:"image_url"`
	LikesCount int       `json:"likes_count"`
	LikedByMe  bool      `json:"liked_by_me"`
	Comments   []Comment `json:"comments"`
	CreatedAt  time.Time `json:"created_at"`
}

type Comment struct {
	ID        int       `json:"id"`
	PostID    int       `json:"post_id"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./sns.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	initDB()

	_ = os.MkdirAll("./uploads/posts", os.ModePerm)
	_ = os.MkdirAll("./uploads/avatars", os.ModePerm)

	mux := http.NewServeMux()
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))
	mux.HandleFunc("/api/posts", handlePosts)
	mux.HandleFunc("/api/posts/delete", handlePostDelete)
	mux.HandleFunc("/api/likes", handleLike)
	mux.HandleFunc("/api/comments", handleComments)
	mux.HandleFunc("/api/users/profile", handleProfile)

	corsMux := corsMiddleware(mux)

	log.Println("サーバーをポート8080で起動中...")
	if err := http.ListenAndServe(":8080", corsMux); err != nil {
		log.Fatal(err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// リクエスト元のドメインを動的に許可し、Codespacesのプロキシを通過させる
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Id, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true") // クッキー認証を許可
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getUserID(r *http.Request) int {
	idStr := r.Header.Get("X-User-Id")
	if idStr == "" {
		idStr = r.URL.Query().Get("user_id")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return 1
	}
	return id
}

func initDB() {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT,
			avatar_url TEXT,
			bio TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			content TEXT,
			image_url TEXT,
			created_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS likes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			post_id INTEGER,
			UNIQUE(user_id, post_id)
		);`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER,
			user_id INTEGER,
			content TEXT,
			created_at DATETIME
		);`,
	}

	for _, stmt := range statements {
		_, err := db.Exec(stmt)
		if err != nil {
			log.Fatal(err)
		}
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count == 0 {
		db.Exec(`INSERT INTO users (id, username, avatar_url, bio) VALUES 
			(1, 'たろう', 'https://api.dicebear.com/7.x/adventurer/svg?seed=taro', 'こんにちは、たろうです。'),
			(2, 'じろう', 'https://api.dicebear.com/7.x/adventurer/svg?seed=jiro', 'じろうです。よろしくお願いします！'),
			(3, 'さぶろう', 'https://api.dicebear.com/7.x/adventurer/svg?seed=sabu', '旅行が趣味です。')`)
	}
}

func handlePosts(w http.ResponseWriter, r *http.Request) {
	currentUserID := getUserID(r)

	if r.Method == "GET" {
		targetUserIDStr := r.URL.Query().Get("user_id")
		query := `SELECT p.id, p.user_id, u.username, u.avatar_url, p.content, p.image_url, p.created_at 
		          FROM posts p JOIN users u ON p.user_id = u.id`
		var rows *sql.Rows
		var err error

		if targetUserIDStr != "" {
			query += " WHERE p.user_id = ? ORDER BY p.created_at DESC"
			rows, err = db.Query(query, targetUserIDStr)
		} else {
			query += " ORDER BY p.created_at DESC"
			rows, err = db.Query(query)
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var posts []Post
		for rows.Next() {
			var p Post
			err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.AvatarURL, &p.Content, &p.ImageURL, &p.CreatedAt)
			if err != nil {
				continue
			}

			db.QueryRow("SELECT COUNT(*) FROM likes WHERE post_id = ?", p.ID).Scan(&p.LikesCount)
			var liked int
			db.QueryRow("SELECT COUNT(*) FROM likes WHERE post_id = ? AND user_id = ?", p.ID, currentUserID).Scan(&liked)
			p.LikedByMe = liked > 0

			cRows, _ := db.Query(`SELECT c.id, c.post_id, u.username, u.avatar_url, c.content, c.created_at 
			                      FROM comments c JOIN users u ON c.user_id = u.id WHERE c.post_id = ? ORDER BY c.created_at ASC`, p.ID)
			p.Comments = []Comment{}
			for cRows.Next() {
				var c Comment
				cRows.Scan(&c.ID, &c.PostID, &c.Username, &c.AvatarURL, &c.Content, &c.CreatedAt)
				p.Comments = append(p.Comments, c)
			}
			cRows.Close()

			posts = append(posts, p)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(posts)

	} else if r.Method == "POST" {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, "Multipart parse error: "+err.Error(), http.StatusBadRequest)
			return
		}

		content := r.FormValue("content")
		imageURL := ""

		file, handler, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), handler.Filename)
			savePath := filepath.Join("./uploads/posts", filename)
			out, err := os.Create(savePath)
			if err == nil {
				io.Copy(out, file)
				imageURL = "/uploads/posts/" + filename
			}
			out.Close()
		}

		_, err = db.Exec("INSERT INTO posts (user_id, content, image_url, created_at) VALUES (?, ?, ?, ?)",
			currentUserID, content, imageURL, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func handlePostDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	currentUserID := getUserID(r)
	postID := r.URL.Query().Get("post_id")

	var imgURL string
	var userID int
	err := db.QueryRow("SELECT user_id, image_url FROM posts WHERE id = ?", postID).Scan(&userID, &imgURL)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	if userID != currentUserID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if imgURL != "" {
		_ = os.Remove("." + imgURL)
	}

	_, _ = db.Exec("DELETE FROM posts WHERE id = ?", postID)
	_, _ = db.Exec("DELETE FROM likes WHERE post_id = ?", postID)
	_, _ = db.Exec("DELETE FROM comments WHERE post_id = ?", postID)

	w.WriteHeader(http.StatusOK)
}

func handleLike(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	currentUserID := getUserID(r)

	var req struct {
		PostID int `json:"post_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM likes WHERE user_id = ? AND post_id = ?", currentUserID, req.PostID).Scan(&count)

	if count > 0 {
		db.Exec("DELETE FROM likes WHERE user_id = ? AND post_id = ?", currentUserID, req.PostID)
	} else {
		db.Exec("INSERT INTO likes (user_id, post_id) VALUES (?, ?)", currentUserID, req.PostID)
	}
	w.WriteHeader(http.StatusOK)
}

func handleComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	currentUserID := getUserID(r)

	var req struct {
		PostID  int    `json:"post_id"`
		Content string `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	_, err := db.Exec("INSERT INTO comments (post_id, user_id, content, created_at) VALUES (?, ?, ?, ?)",
		req.PostID, currentUserID, req.Content, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	currentUserID := getUserID(r)

	if r.Method == "GET" {
		var u User
		err := db.QueryRow("SELECT id, username, avatar_url, bio FROM users WHERE id = ?", currentUserID).Scan(&u.ID, &u.Username, &u.AvatarURL, &u.Bio)
		if err != nil {
			_, _ = db.Exec("INSERT INTO users (id, username, avatar_url, bio) VALUES (?, ?, ?, ?)", currentUserID, "ゲストユーザー", "https://api.dicebear.com/7.x/adventurer/svg?seed=guest", "プロフィール未設定")
			u = User{ID: currentUserID, Username: "ゲストユーザー", AvatarURL: "https://api.dicebear.com/7.x/adventurer/svg?seed=guest", Bio: "プロフィール未設定"}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)

	} else if r.Method == "POST" {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "Profile form parse error: "+err.Error(), http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		bio := r.FormValue("bio")

		var currentAvatar string
		_ = db.QueryRow("SELECT avatar_url FROM users WHERE id = ?", currentUserID).Scan(&currentAvatar)

		avatarURL := currentAvatar
		file, handler, err := r.FormFile("avatar")
		if err == nil {
			defer file.Close()
			filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), handler.Filename)
			savePath := filepath.Join("./uploads/avatars", filename)
			out, err := os.Create(savePath)
			if err == nil {
				io.Copy(out, file)
				avatarURL = "/uploads/avatars/" + filename
				if currentAvatar != "" && !strings.Contains(currentAvatar, "api.dicebear.com") {
					_ = os.Remove("." + currentAvatar)
				}
			}
			out.Close()
		}

		_, err = db.Exec("UPDATE users SET username = ?, bio = ?, avatar_url = ? WHERE id = ?", username, bio, avatarURL, currentUserID)
		if err != nil {
			http.Error(w, "DB Update error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}