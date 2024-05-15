package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

func main() {
	// Open the Excel file
	f, err := excelize.OpenFile("SOFAAMY.xlsx") //name of excel file to open
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	// Get the first worksheet
	sheet := f.GetSheetName(0)

	// Get the merged cells
	mergedCells, err := f.GetMergeCells(sheet)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Unmerge cells
	for _, mc := range mergedCells {
		err = f.UnmergeCell(sheet, mc.GetStartAxis(), mc.GetEndAxis())
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Remove the first 25 rows
	for i := 1; i <= 25; i++ {
		err := f.RemoveRow(sheet, i)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Remove the last 14 merged rows
	lastRow, err := f.GetRows(sheet)
	if err != nil {
		fmt.Println(err)
		return
	}
	for i := len(lastRow) - 14; i < len(lastRow); i++ {
		err := f.RemoveRow(sheet, i+1) // i+1 because row indices are 1-based
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Create a CSV file
	csvFile, err := os.Create("cleaned_data.csv")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Extract only values from columns A, Y, and AL and write them to CSV
	rows, err := f.GetRows(sheet)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, row := range rows {
		if len(row) >= 39 { // Check if the row has at least 39 columns
			// Assuming row[37] contains the "Amount" values in column AL
			amountStr := row[37]

			// Remove commas from the amount string
			amountStr = strings.Replace(amountStr, ",", "", -1)

			// Check if the amount string starts with a negative sign
			if strings.HasPrefix(amountStr, "-") {
				// If yes, remove the negative sign
				amountStr = amountStr[1:]
			}

			// Parse the amount string as a float
			amount, err := strconv.ParseFloat(amountStr, 64)
			if err != nil {
				fmt.Println("Error parsing amount:", err)
				continue
			}

			// Convert the amount back to a string without formatting
			formattedAmount := strconv.FormatFloat(amount, 'f', -1, 64)

			// Columns A, Y, and AL without quotes around the amount
			newRow := []string{row[0], row[24], formattedAmount}
			err = writer.Write(newRow)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}

	fmt.Println("Data extraction and conversion to CSV completed successfully.")
}
