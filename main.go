package handler

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

func init() {
	// Ensure directories exist
	os.MkdirAll(uploadDir, os.ModePerm)
	os.MkdirAll(convertedDir, os.ModePerm)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/" && r.Method == http.MethodGet:
		homeHandler(w, r)
	case r.URL.Path == "/upload" && r.Method == http.MethodPost:
		uploadHandler(w, r)
	case strings.HasPrefix(r.URL.Path, "/download/"):
		downloadHandler(w, r)
	default:
		http.NotFound(w, r)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/index.html")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

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

	ext := filepath.Ext(header.Filename)
	if ext != ".txt" {
		http.Error(w, "Unsupported file type. Only .txt files are allowed.", http.StatusUnsupportedMediaType)
		return
	}

	pdfFileName := strings.TrimSuffix(header.Filename, ext) + ".pdf"
	pdfPath := filepath.Join(convertedDir, pdfFileName)
	err = convertTextToPDF(uploadedFilePath, pdfPath)
	if err != nil {
		http.Error(w, "Failed to convert to PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		time.Sleep(2 * time.Second)
		os.Remove(uploadedFilePath)
	}()

	downloadURL := fmt.Sprintf("/download/%s", pdfFileName)
	http.Redirect(w, r, downloadURL, http.StatusFound)
}

func convertTextToPDF(inputPath, outputPath string) error {
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(190, 10, string(content), "", "L", false)

	return pdf.OutputFileAndClose(outputPath)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	fileName := strings.TrimPrefix(r.URL.Path, "/download/")
	filePath := filepath.Join(convertedDir, fileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	w.Header().Set("Content-Type", "application/pdf")

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	io.Copy(w, file)

	go func() {
		time.Sleep(2 * time.Second)
		os.Remove(filePath)
	}()
}
