package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

const uploadDir = "./uploads"
const convertedDir = "./converted"

func main() {
	// Ensure directories exist
	os.MkdirAll(uploadDir, os.ModePerm)
	os.MkdirAll(convertedDir, os.ModePerm)

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/download/", downloadHandler) // Custom download handler
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	fmt.Println("Server started on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/index.html")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save uploaded file
	uploadedFilePath := filepath.Join(uploadDir, header.Filename)
	uploadedFile, err := os.Create(uploadedFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
		return
	}
	defer uploadedFile.Close()

	_, err = io.Copy(uploadedFile, file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if file is .txt
	ext := filepath.Ext(header.Filename)
	if ext != ".txt" {
		http.Error(w, "Unsupported file type. Only .txt files are allowed.", http.StatusUnsupportedMediaType)
		return
	}

	// Convert .txt to .pdf
	pdfFileName := strings.TrimSuffix(header.Filename, ext) + ".pdf"
	pdfPath := filepath.Join(convertedDir, pdfFileName)
	err = convertTextToPDF(uploadedFilePath, pdfPath)
	if err != nil {
		http.Error(w, "Failed to convert to PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Defer deletion of the uploaded file after 2 seconds
	go func() {
		time.Sleep(2 * time.Second)
		err := os.Remove(uploadedFilePath)
		if err != nil {
			fmt.Printf("Failed to delete uploaded file %s: %v\n", uploadedFilePath, err)
		}
	}()

	// Redirect to the download URL
	downloadURL := fmt.Sprintf("/download/%s", pdfFileName)
	http.Redirect(w, r, downloadURL, http.StatusFound)
}

// Convert Text File to PDF using gofpdf
func convertTextToPDF(inputPath, outputPath string) error {
	// Read the content of the input file
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	// Create a new PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)

	// Add content to PDF
	pdf.MultiCell(190, 10, string(content), "", "L", false)

	// Save the PDF
	return pdf.OutputFileAndClose(outputPath)
}

// Handle downloading the file and deleting it
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	fileName := strings.TrimPrefix(r.URL.Path, "/download/")
	filePath := filepath.Join(convertedDir, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	w.Header().Set("Content-Type", "application/pdf")

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Stream the file content
	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, "Failed to send file", http.StatusInternalServerError)
		return
	}

	// Defer deletion of the converted file after 2 seconds
	go func() {
		time.Sleep(2 * time.Second)
		err := os.Remove(filePath)
		if err != nil {
			fmt.Printf("Failed to delete converted file %s: %v\n", filePath, err)
		}
	}()
}
