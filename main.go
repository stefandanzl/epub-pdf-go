package main

import (
	"bytes"
	"fmt"
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Loading environment variables")
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	log.Printf("Initializing WebDAV client: URL=%s, Username=%s",
		os.Getenv("WEBDAV_URL"),
		os.Getenv("WEBDAV_USERNAME"))

	webdavClient = gowebdav.NewClient(
		os.Getenv("WEBDAV_URL"),
		os.Getenv("WEBDAV_USERNAME"),
		os.Getenv("WEBDAV_PASSWORD"),
	)

	log.Printf("Creating temp directory: %s", tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default())

	clients := make(map[*gin.Context]bool)

	r.Static("/static", "./public")
	log.Println("Configured static file serving from ./public")

	r.GET("/status", func(c *gin.Context) {
		log.Printf("New SSE client connected: %s", c.ClientIP())
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
		log.Printf("Received conversion request from %s", c.ClientIP())

		var req struct {
			EpubURL string `json:"epubUrl"`
		}
		if err := c.BindJSON(&req); err != nil {
			log.Printf("Failed to parse request: %v", err)
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		fileName := path.Base(req.EpubURL)
		tempPath := path.Join(tempDir, fileName)
		pdfPath := tempPath[:len(tempPath)-5] + ".pdf"
		log.Printf("Processing files: EPUB=%s, PDF=%s", tempPath, pdfPath)

		// Download
		log.Printf("Downloading EPUB from: %s", req.EpubURL)
		broadcast(clients, gin.H{"step": 1, "status": "downloading"})

		resp, err := http.Get(req.EpubURL)
		if err != nil {
			log.Printf("Download failed: %v", err)
			c.JSON(500, gin.H{"error": fmt.Sprintf("Download failed: %v", err)})
			return
		}
		defer resp.Body.Close()

		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, resp.Body); err != nil {
			log.Printf("Failed to read response body: %v", err)
			c.JSON(500, gin.H{"error": fmt.Sprintf("Download failed: %v", err)})
			return
		}

		if err := os.WriteFile(tempPath, buf.Bytes(), 0644); err != nil {
			log.Printf("Failed to save EPUB: %v", err)
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to save file: %v", err)})
			return
		}
		log.Printf("EPUB downloaded successfully: %d bytes", buf.Len())
		broadcast(clients, gin.H{"step": 2, "status": "downloaded"})

		// Convert
		log.Println("Starting conversion to PDF")
		broadcast(clients, gin.H{"step": 3, "status": "converting"})

		cmd := exec.Command("ebook-convert", tempPath, pdfPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Conversion failed: %v\nOutput: %s", err, string(output))
			c.JSON(500, gin.H{"error": fmt.Sprintf("Conversion failed: %v", err)})
			return
		}
		log.Println("Conversion completed successfully")

		// Upload
		log.Println("Uploading to WebDAV")
		broadcast(clients, gin.H{"step": 4, "status": "uploading"})

		pdfData, err := os.ReadFile(pdfPath)
		if err != nil {
			log.Printf("Failed to read PDF: %v", err)
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to read PDF: %v", err)})
			return
		}

		webdavPath := "/" + path.Base(pdfPath)
		if err := webdavClient.Write(webdavPath, pdfData, 0644); err != nil {
			log.Printf("WebDAV upload failed: %v", err)
			c.JSON(500, gin.H{"error": fmt.Sprintf("Upload failed: %v", err)})
			return
		}
		log.Printf("PDF uploaded successfully to %s", webdavPath)

		// Cleanup
		log.Println("Cleaning up temporary files")
		broadcast(clients, gin.H{"step": 5, "status": "cleaning"})

		if err := os.Remove(tempPath); err != nil {
			log.Printf("Failed to remove EPUB: %v", err)
		}
		if err := os.Remove(pdfPath); err != nil {
			log.Printf("Failed to remove PDF: %v", err)
		}

		broadcast(clients, gin.H{"status": "complete"})
		c.JSON(200, gin.H{"status": "success"})
		log.Println("Conversion process completed successfully")
	})

	log.Printf("Server starting on :3000")
	log.Fatal(r.Run(":3000"))
}

func broadcast(clients map[*gin.Context]bool, data gin.H) {
	log.Printf("Broadcasting to %d clients: %v", len(clients), data)
	for client := range clients {
		client.SSEvent("message", data)
	}
}
