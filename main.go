package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/studio-b12/gowebdav"
)

var (
	webdavClient *gowebdav.Client
	tempDir      = "./temp"
)

func init() {
	godotenv.Load()

	webdavClient = gowebdav.NewClient(
		os.Getenv("WEBDAV_URL"),
		os.Getenv("WEBDAV_USERNAME"),
		os.Getenv("WEBDAV_PASSWORD"),
	)

	os.MkdirAll(tempDir, 0755)
}

func main() {
	r := gin.Default()
	r.Use(cors.Default())
	r.StaticFS("/", http.Dir("public"))

	clients := make(map[*gin.Context]bool)

	r.GET("/status", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		clients[c] = true
		defer delete(clients, c)

		c.Stream(func(w io.Writer) bool {
			return true
		})
	})

	r.POST("/convert", func(c *gin.Context) {
		var req struct {
			EpubURL string `json:"epubUrl"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		fileName := path.Base(req.EpubURL)
		tempPath := path.Join(tempDir, fileName)
		pdfPath := tempPath[:len(tempPath)-5] + ".pdf"

		broadcast(clients, gin.H{"step": 1, "status": "downloading"})

		resp, err := http.Get(req.EpubURL)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, resp.Body); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		if err := os.WriteFile(tempPath, buf.Bytes(), 0644); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		broadcast(clients, gin.H{"step": 2, "status": "downloaded"})

		broadcast(clients, gin.H{"step": 3, "status": "converting"})
		cmd := exec.Command("ebook-convert", tempPath, pdfPath)
		if err := cmd.Run(); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		broadcast(clients, gin.H{"step": 4, "status": "uploading"})
		pdfData, err := os.ReadFile(pdfPath)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		if err := webdavClient.Write("/"+path.Base(pdfPath), pdfData, 0644); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		broadcast(clients, gin.H{"step": 5, "status": "cleaning"})
		os.Remove(tempPath)
		os.Remove(pdfPath)

		broadcast(clients, gin.H{"status": "complete"})
		c.JSON(200, gin.H{"status": "success"})
	})

	log.Fatal(r.Run(":3000"))
}

func broadcast(clients map[*gin.Context]bool, data gin.H) {
	for client := range clients {
		client.SSEvent("message", data)
	}
}
