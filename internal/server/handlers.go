package server

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project/internal/models"
)

func (s *Server) UploadPrices(w http.ResponseWriter, r *http.Request) {
	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/zip" {
		http.Error(w, "Expected zip file, got: "+contentType, http.StatusBadRequest)
		return
	}

	// Read the zip file from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	// Process the zip file
	stats, err := s.processZipFile(body)
	if err != nil {
		log.Printf("Error processing zip file: %v", err)
		http.Error(w, "Error processing zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return statistics
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) processZipFile(zipData []byte) (*models.UploadStats, error) {
	// Create zip reader from memory
	zipReader, err := zip.NewReader(strings.NewReader(string(zipData)), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %v", err)
	}

	// Find CSV file (ищем в любом месте архива)
	var csvFile *zip.File
	for _, file := range zipReader.File {
		// Ищем любой CSV файл, независимо от пути
		if strings.HasSuffix(file.Name, ".csv") {
			csvFile = file
			break
		}
	}

	if csvFile == nil {
		// Покажем какие файлы есть в архиве для отладки
		fileList := make([]string, len(zipReader.File))
		for i, file := range zipReader.File {
			fileList[i] = file.Name
		}
		return nil, fmt.Errorf("CSV file not found in zip. Files in archive: %v", fileList)
	}

	log.Printf("Found CSV file: %s", csvFile.Name)

	// Open and read CSV file
	fileReader, err := csvFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open csv: %v", err)
	}
	defer fileReader.Close()

	// Parse CSV and insert into database
	return s.parseAndInsertCSV(fileReader)
}

func (s *Server) parseAndInsertCSV(reader io.Reader) (*models.UploadStats, error) {
	csvReader := csv.NewReader(reader)
	
	stats := &models.UploadStats{}
	categories := make(map[string]bool)

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Prepare insert statement
	stmt, err := tx.Prepare(`
		INSERT INTO prices (product_id, created_date, product_name, category, price) 
		VALUES ($1, $2, $3, $4, $5)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Read CSV records
	lineNumber := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record at line %d: %v", lineNumber+1, err)
		}

		lineNumber++

		// Skip empty lines
		if len(record) == 0 {
			continue
		}

		// Skip header row (проверяем если первое поле выглядит как заголовок)
		if lineNumber == 1 {
			// Проверяем если первое поле содержит "id" (заголовок)
			if record[0] == "id" || strings.Contains(strings.ToLower(record[0]), "id") {
				continue // Пропускаем заголовок
			}
		}

		// Parse record (5 fields expected)
		if len(record) != 5 {
			return nil, fmt.Errorf("invalid CSV format at line %d: expected 5 fields, got %d", lineNumber, len(record))
		}

		priceRecord, err := parseCSVRecord(record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse record at line %d: %v", lineNumber, err)
		}

		// Insert into database
		_, err = stmt.Exec(
			priceRecord.ProductID,
			priceRecord.CreatedDate,
			priceRecord.ProductName,
			priceRecord.Category,
			priceRecord.Price,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert record at line %d: %v", lineNumber, err)
		}

		// Update statistics
		stats.TotalItems++
		stats.TotalPrice += priceRecord.Price
		categories[priceRecord.Category] = true
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	stats.TotalCategories = len(categories)
	return stats, nil
}

func parseCSVRecord(record []string) (*models.PriceRecord, error) {
	// Парсим согласно реальной структуре CSV:
	// [0] = id (product_id)
	// [1] = name (product_name) 
	// [2] = category
	// [3] = price
	// [4] = create_date (created_date)

	// Parse date (format: ГОД-МЕСЯЦ-ДЕНЬ)
	createdDate, err := time.Parse("2006-01-02", record[4])
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %s", record[4])
	}

	// Parse price
	price, err := strconv.ParseFloat(record[3], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid price format: %s", record[3])
	}

	return &models.PriceRecord{
		ProductID:   record[0],    // id
		CreatedDate: createdDate,  // create_date
		ProductName: record[1],    // name
		Category:    record[2],    // category
		Price:       price,        // price
	}, nil
}

func (s *Server) DownloadPrices(w http.ResponseWriter, r *http.Request) {
	// Get all data from database
	rows, err := s.db.Query(`
		SELECT product_id, created_date, product_name, category, price 
		FROM prices 
		ORDER BY id
	`)
	if err != nil {
		http.Error(w, "Failed to query database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Create CSV data
	var csvData strings.Builder
	csvData.WriteString("product_id,created_date,product_name,category,price\n")

	for rows.Next() {
		var productID, productName, category string
		var createdDate time.Time
		var price float64

		err := rows.Scan(&productID, &createdDate, &productName, &category, &price)
		if err != nil {
			http.Error(w, "Failed to scan row: "+err.Error(), http.StatusInternalServerError)
			return
		}

		csvData.WriteString(fmt.Sprintf("%s,%s,%s,%s,%.2f\n",
			productID,
			createdDate.Format("2006-01-02"),
			productName,
			category,
			price,
		))
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Error iterating rows: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create ZIP archive in memory
	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)

	// Add CSV file to ZIP
	csvWriter, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Failed to create CSV in zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = csvWriter.Write([]byte(csvData.String()))
	if err != nil {
		http.Error(w, "Failed to write CSV to zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Close ZIP writer
	err = zipWriter.Close()
	if err != nil {
		http.Error(w, "Failed to close zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"prices.zip\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", zipBuffer.Len()))

	// Send ZIP file
	_, err = w.Write(zipBuffer.Bytes())
	if err != nil {
		log.Printf("Failed to send zip file: %v", err)
	}
}
