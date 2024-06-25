package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

var db *gorm.DB

func main() {
	// Initialize Gin router
	r := gin.Default()

	// Connect to SQLite database
	var err error
	db, err = gorm.Open("sqlite3", "database.db")
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Auto create tables based on struct models
	db.AutoMigrate(&File{})

	// Routes
	r.GET("/", indexHandler)
	r.POST("/upload", uploadFile)
	r.GET("/files", listFiles)

	// Serve static files (CSS, JS, etc.)
	r.Static("/static", "./static")

	// Start the server
	r.Run(":8080")
}

// File model
type File struct {
	gorm.Model
	FileName string
	FileType string
}

// Function to convert to PDF
func convertToPDF(inputFile, outputFile string) error {
	cmd := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", "--outdir", "./output", inputFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("conversion failed: %v", err)
	}
	return nil
}

// Handler for index page
func indexHandler(c *gin.Context) {
	var files []File
	db.Find(&files)
	c.HTML(http.StatusOK, "index.html", gin.H{
		"files": files,
	})
}

// Handler to upload files
func uploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}

	// Save the uploaded file
	filepath := "uploads/" + file.Filename
	if err := c.SaveUploadedFile(file, filepath); err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// Determine file type
	var fileType string
	switch {
	case filepath[len(filepath)-5:] == ".docx":
		fileType = "docx"
	case filepath[len(filepath)-5:] == ".pptx":
		fileType = "pptx"
	default:
		c.String(http.StatusBadRequest, "unsupported file type")
		return
	}

	// Convert to PDF
	outputFilename := "output/" + file.Filename + ".pdf"
	if err := convertToPDF(filepath, outputFilename); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("conversion error: %v", err))
		return
	}

	// Save file details to database
	db.Create(&File{
		FileName: file.Filename,
		FileType: fileType,
	})

	// Redirect back to index page
	c.Redirect(http.StatusSeeOther, "/")
}

// Handler to list files
func listFiles(c *gin.Context) {
	var files []File
	db.Find(&files)
	c.JSON(http.StatusOK, gin.H{"files": files})
}
