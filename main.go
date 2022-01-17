package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/pluveto/pastex2/pkg/lang_detector"
)

var (
	rdb *redis.Client
)

// 初始化连接
func initRedis() (err error) {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",  // no password set
		DB:       0,   // use default DB
		PoolSize: 100, // 连接池大小
	})

	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = rdb.Ping().Result()
	return err
}
func init() {
	log.SetFlags(log.Llongfile | log.LstdFlags)

	rand.Seed(time.Now().UnixNano())
	lang_detector.LoadRules()
}

type Post struct {
	lang      string
	tsCreated int64
	tsUpdated int64
	pass      string
	title     string
	text      string
}

func extractPost(cache string) Post {
	var post Post
	// parse cache
	scanner := bufio.NewScanner(strings.NewReader(cache))

	scanner.Scan()
	post.lang = scanner.Text()

	scanner.Scan()
	tsCreated, _ := strconv.ParseInt(scanner.Text(), 16, 64)
	post.tsCreated = tsCreated

	scanner.Scan()
	tsUpdated, _ := strconv.ParseInt(scanner.Text(), 16, 64)
	post.tsUpdated = tsUpdated

	scanner.Scan()
	post.pass = scanner.Text()

	scanner.Scan()
	post.title = scanner.Text()

	var text bytes.Buffer

	for scanner.Scan() {
		text.WriteString(scanner.Text() + "\n")
	}
	post.text = text.String()
	return post
}

func main() {

	if err := initRedis(); err != nil {
		os.Exit(1)
	}

	router := gin.Default()
	router.Delims("{{", "}}")
	router.LoadHTMLFiles("./templates/home.tmpl",
		"./templates/share.tmpl",
		"./templates/edit.tmpl",
		"./templates/notfound.tmpl")
	router.Static("/assets", "./assets")
	router.StaticFile("/favicon.ico", "./assets/favicon.png")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "home.tmpl", map[string]interface{}{
			"theme": "prism-tomorrow",
		})
	})
	router.GET("/share/:sid/raw", func(c *gin.Context) {
		sid := c.Param("sid")
		cache, err := rdb.Get(sid).Result()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"msg": "notfound", "code": 40400})
			return
		}
		post := extractPost(cache)
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, post.text)
	})
	router.GET("/share/:sid/edit", func(c *gin.Context) {
		sid := c.Param("sid")
		cache, err := rdb.Get(sid).Result()
		if err != nil {
			c.HTML(http.StatusNotFound, "notfound.tmpl", nil)
			return
		}
		post := extractPost(cache)
		created_str := time.Unix(post.tsCreated, 0).Format(time.RFC3339)
		updated_str := time.Unix(post.tsUpdated, 0).Format(time.RFC3339)
		c.HTML(http.StatusOK, "edit.tmpl", map[string]interface{}{
			"title":       post.title,
			"lang_text":   post.lang,
			"created_str": created_str,
			"created":     post.tsCreated,
			"updated_str": updated_str,
			"updated":     post.tsUpdated,
			"lang":        post.lang,
			"text":        post.text,
		})
	})
	router.GET("/share/:sid", func(c *gin.Context) {
		sid := c.Param("sid")
		cache, err := rdb.Get(sid).Result()
		if err != nil {
			c.HTML(http.StatusNotFound, "notfound.tmpl", nil)
			return
		}
		post := extractPost(cache)
		created_str := time.Unix(post.tsCreated, 0).Format(time.RFC3339)
		updated_str := time.Unix(post.tsUpdated, 0).Format(time.RFC3339)
		c.HTML(http.StatusOK, "share.tmpl", map[string]interface{}{
			"title":       post.title,
			"lang_text":   post.lang,
			"created_str": created_str,
			"created":     post.tsCreated,
			"updated_str": updated_str,
			"updated":     post.tsUpdated,
			"lang":        post.lang,
			"text":        post.text,
		})
	})

	langs := map[string]bool{
		"none": true,
		"c":    true,
		"cpp":  true,
		"css":  true,
		"go":   true,
		"html": true,
		"java": true,
		"js":   true,
		"md":   true,
		"php":  true,
		"py":   true,
		"ruby": true,
	}
	router.POST("share", func(c *gin.Context) {
		pass := RandStringRunes(64)
		sid := fmt.Sprintf("%x", sha256.Sum256([]byte(pass)))[0:16]
		text := c.PostForm("text")
		if len(strings.TrimSpace(text)) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "empty text", "code": 40001})
			return
		}
		title := c.PostForm("title")
		if len(strings.TrimSpace(title)) == 0 {
			title = "No title"
		}
		lang := c.PostForm("lang")
		// TODO:
		// filename := c.PostForm("filename")
		if lang == "auto" {
			langName := lang_detector.Detect(text)
			if len(langName) == 0 {
				lang = "none"
				log.Printf("Failed to detect language")
			} else {
				fmt.Printf("found: %s", langName)
				lang = strings.ToLower(langName)
			}
		}
		if !langs[lang] {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "unsupported lang", "code": 40002, "lang": lang})
			return
		}
		tsCreated := time.Now().Unix()
		tsUpdated := int64(0)
		store :=
			/*     lang         */ lang + "\n" +
				/* time created */ strconv.FormatInt(tsCreated, 16) + "\n" +
				/* time updated */ strconv.FormatInt(tsUpdated, 16) + "\n" +
				/* password     */ pass + "\n" +
				/* title        */ title + "\n" +
				/* text         */ text
		if err := rdb.Set(sid, store, 30*24*time.Hour).Err(); err != nil {
			fmt.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "redis error", "code": 50000})
			return
		}

		c.JSON(200, gin.H{"sid": sid, "pass": pass})
	})

	router.Run("0.0.0.0:8080")
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
