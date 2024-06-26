package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/jung-kurt/gofpdf"
	"github.com/russross/blackfriday/v2"
	"github.com/tealeg/xlsx"
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

	// Load HTML templates
	r.LoadHTMLGlob("templates/*.html")

	// Ensure directories exist
	ensureDir("uploads")
	ensureDir("output")

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

// Function to ensure a directory exists
func ensureDir(dirName string) {
	err := os.MkdirAll(dirName, os.ModePerm)
	if err != nil {
		log.Fatalf("Error creating directory %s: %v", dirName, err)
	}
}

// Function to convert to PDF
func convertToPDF(inputFile, outputFile string, fileType string) error {
	switch fileType {
	case "docx", "pptx":
		cmd := exec.Command("soffice", "--headless", "--convert-to", "pdf", "--outdir", "./output", inputFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "xlsx":
		return convertExcelToPDF(inputFile, outputFile)
	case "csv":
		return convertCSVToPDF(inputFile, outputFile)
	case "txt":
		return convertTextToPDF(inputFile, outputFile)
	case "md":
		return convertMarkdownToPDF(inputFile, outputFile)
	default:
		return fmt.Errorf("unsupported file type: %s", fileType)
	}
}

// Function to convert Excel to PDF
func convertExcelToPDF(inputFile, outputFile string) error {
	xlFile, err := xlsx.OpenFile(inputFile)
	if err != nil {
		return err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	for _, sheet := range xlFile.Sheets {
		for _, row := range sheet.Rows {
			for _, cell := range row.Cells {
				text := cell.String()
				pdf.CellFormat(190, 10, text, "", 0, "L", false, 0, "")
			}
			pdf.Ln(12)
		}
	}
	return pdf.OutputFileAndClose(outputFile)
}

// Function to convert CSV to PDF
func convertCSVToPDF(inputFile, outputFile string) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(190, 10, string(data), "", "L", false)

	return pdf.OutputFileAndClose(outputFile)
}

// Function to convert text to PDF
func convertTextToPDF(inputFile, outputFile string) error {
	data, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(190, 10, string(data), "", "L", false)

	return pdf.OutputFileAndClose(outputFile)
}

// Function to convert markdown to PDF
func convertMarkdownToPDF(inputFile, outputFile string) error {
	data, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return err
	}

	html := blackfriday.Run(data)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	htmlStr := string(html)

	htmlParser := pdf.HTMLBasicNew()
	htmlParser.Write(10, htmlStr)

	return pdf.OutputFileAndClose(outputFile)
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
	fileType := determineFileType(filepath)

	// Convert to PDF
	outputFilename := "output/" + file.Filename + ".pdf"
	if err := convertToPDF(filepath, outputFilename, fileType); err != nil {
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

// Function to determine file type
func determineFileType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".docx":
		return "docx"
	case ".pptx":
		return "pptx"
	case ".xlsx":
		return "xlsx"
	case ".csv":
		return "csv"
	case ".txt":
		return "txt"
	case ".md":
		return "md"
	default:
		return "unknown"
	}
}

// Handler to list files
func listFiles(c *gin.Context) {
	var files []File
	db.Find(&files)
	c.JSON(http.StatusOK, gin.H{"files": files})
}
